package speech

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/cycl0o0/GPTerminal/internal/config"
	"golang.org/x/net/websocket"
)

const (
	DefaultRealtimeSessionModel = "gpt-realtime"
	realtimeAPIURL              = "wss://api.openai.com/v1/realtime"
	realtimeOrigin              = "http://localhost"
	audioChunkBytes             = 4800
	minCommitAudioBytes         = audioChunkBytes
	finalFlushTimeout           = 1500 * time.Millisecond
)

var lookPath = exec.LookPath
var runCommandOutput = func(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).CombinedOutput()
	return string(out), err
}

type RealtimeTranscriptionOptions struct {
	SessionModel       string
	TranscriptionModel string
	Language           string
	Prompt             string
	Recorder           string
	Device             string
	OutputPath         string
}

type RealtimeTranscriptionResult struct {
	OutputPath string
	Turns      int
}

type realtimeEvent struct {
	Type         string          `json:"type"`
	ItemID       string          `json:"item_id"`
	ContentIndex int             `json:"content_index"`
	Delta        string          `json:"delta"`
	Transcript   string          `json:"transcript"`
	Error        *realtimeErrMsg `json:"error"`
}

type realtimeErrMsg struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Type    string `json:"type"`
}

type transcriptState struct {
	printed bool
	buf     strings.Builder
}

type recorderSpec struct {
	name string
	args []string
}

type streamStats struct {
	audioBytesSent int
}

type transcriptWriter struct {
	mu    sync.Mutex
	file  *os.File
	turns int
}

func TranscribeMicrophoneRealtime(ctx context.Context, opts RealtimeTranscriptionOptions) (*RealtimeTranscriptionResult, error) {
	key := config.APIKey()
	if key == "" {
		return nil, fmt.Errorf("OpenAI API key not set. Run: gpterminal config set-key <key>\nOr set OPENAI_API_KEY environment variable")
	}

	transcriptionModel := strings.TrimSpace(opts.TranscriptionModel)
	if transcriptionModel == "" {
		transcriptionModel = DefaultTranscriptionModel
	}

	ws, err := dialRealtimeWebSocket(ctx, key)
	if err != nil {
		return nil, err
	}
	defer ws.Close()

	if err := websocket.JSON.Send(ws, buildRealtimeSessionUpdate(transcriptionModel, opts.Language, opts.Prompt)); err != nil {
		return nil, fmt.Errorf("send realtime session config: %w", err)
	}

	writer, err := newTranscriptWriter(opts.OutputPath)
	if err != nil {
		return nil, err
	}
	defer writer.Close()

	readerErrCh := make(chan error, 1)
	go func() {
		readerErrCh <- readRealtimeTranscriptionEvents(ctx, ws, writer)
	}()

	streamErr := streamMicrophoneAudio(ctx, ws, opts.Recorder, opts.Device)
	if streamErr != nil && !errors.Is(streamErr, context.Canceled) {
		_ = ws.Close()
		<-readerErrCh
		return nil, streamErr
	}

	time.Sleep(finalFlushTimeout)
	_ = ws.Close()

	readerErr := <-readerErrCh
	if readerErr != nil && !errors.Is(readerErr, io.EOF) && !errors.Is(readerErr, context.Canceled) && !isClosedConnErr(readerErr) {
		return nil, readerErr
	}

	return &RealtimeTranscriptionResult{
		OutputPath: opts.OutputPath,
		Turns:      writer.Turns(),
	}, nil
}

func dialRealtimeWebSocket(ctx context.Context, apiKey string) (*websocket.Conn, error) {
	values := url.Values{}
	values.Set("intent", "transcription")

	u := realtimeAPIURL + "?" + values.Encode()
	cfg, err := websocket.NewConfig(u, realtimeOrigin)
	if err != nil {
		return nil, fmt.Errorf("build realtime websocket config: %w", err)
	}
	cfg.Header.Set("Authorization", "Bearer "+apiKey)

	ws, err := cfg.DialContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("connect realtime websocket: %w", err)
	}
	return ws, nil
}

func buildRealtimeSessionUpdate(model, language, prompt string) map[string]any {
	transcription := map[string]any{
		"model": model,
	}
	if strings.TrimSpace(language) != "" {
		transcription["language"] = strings.TrimSpace(language)
	}
	if strings.TrimSpace(prompt) != "" {
		transcription["prompt"] = strings.TrimSpace(prompt)
	}

	return map[string]any{
		"type": "session.update",
		"session": map[string]any{
			"type": "transcription",
			"audio": map[string]any{
				"input": map[string]any{
					"format": map[string]any{
						"type": "audio/pcm",
						"rate": 24000,
					},
					"noise_reduction": map[string]any{
						"type": "near_field",
					},
					"transcription": transcription,
					"turn_detection": map[string]any{
						"type":                "server_vad",
						"threshold":           0.5,
						"prefix_padding_ms":   300,
						"silence_duration_ms": 500,
					},
				},
			},
		},
	}
}

func streamMicrophoneAudio(ctx context.Context, ws *websocket.Conn, recorder, device string) error {
	spec, err := detectRecorder(recorder, device)
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, spec.name, spec.args...)
	cmd.Stderr = os.Stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("open microphone capture pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start microphone recorder %q: %w", spec.name, err)
	}

	stats, readErr := streamAudioChunks(ctx, ws, stdout)
	waitErr := cmd.Wait()

	if ctx.Err() != nil {
		switch {
		case stats.audioBytesSent == 0:
			return fmt.Errorf("no microphone audio was captured before stop; try --recorder pw-record or --recorder parec, and optionally --device")
		case stats.audioBytesSent < minCommitAudioBytes:
			return fmt.Errorf("captured only %.0f ms of audio before stop; speak a bit longer before pressing Ctrl+C", millisForAudioBytes(stats.audioBytesSent))
		}
	}
	if readErr != nil && !errors.Is(readErr, context.Canceled) && !errors.Is(readErr, io.EOF) {
		return readErr
	}
	if waitErr != nil && ctx.Err() == nil {
		return fmt.Errorf("microphone recorder exited: %w", waitErr)
	}
	return ctx.Err()
}

func detectRecorder(preference, device string) (*recorderSpec, error) {
	switch strings.ToLower(strings.TrimSpace(preference)) {
	case "", "auto":
		switch runtime.GOOS {
		case "darwin":
			if _, err := lookPath("sox"); err == nil {
				return buildSoxDarwinSpec(device), nil
			}
			if _, err := lookPath("ffmpeg"); err == nil {
				return buildFFmpegDarwinSpec(device), nil
			}
			return nil, fmt.Errorf("no supported microphone recorder found on macOS; install sox (brew install sox) or ffmpeg")
		case "windows":
			if _, err := lookPath("ffmpeg"); err == nil {
				return buildFFmpegWindowsSpec(device), nil
			}
			return nil, fmt.Errorf("no supported microphone recorder found on Windows; install ffmpeg")
		default: // linux and others
			if _, err := lookPath("pw-record"); err == nil {
				return buildPWRecordSpec(device), nil
			}
			if _, err := lookPath("parec"); err == nil {
				return buildParecSpec(device), nil
			}
			if _, err := lookPath("arecord"); err == nil && hasUsableARecordDevice() {
				return buildARecordSpec(device), nil
			}
			if _, err := lookPath("ffmpeg"); err == nil {
				return buildFFmpegSpec(device), nil
			}
			return nil, fmt.Errorf("no supported microphone recorder found; install pw-record, parec, arecord, sox, or ffmpeg")
		}
	case "pw-record":
		if _, err := lookPath("pw-record"); err != nil {
			return nil, fmt.Errorf("pw-record not found in PATH")
		}
		return buildPWRecordSpec(device), nil
	case "parec":
		if _, err := lookPath("parec"); err != nil {
			return nil, fmt.Errorf("parec not found in PATH")
		}
		return buildParecSpec(device), nil
	case "arecord":
		if _, err := lookPath("arecord"); err != nil {
			return nil, fmt.Errorf("arecord not found in PATH")
		}
		return buildARecordSpec(device), nil
	case "sox":
		if _, err := lookPath("sox"); err != nil {
			return nil, fmt.Errorf("sox not found in PATH")
		}
		return buildSoxDarwinSpec(device), nil
	case "ffmpeg":
		if _, err := lookPath("ffmpeg"); err != nil {
			return nil, fmt.Errorf("ffmpeg not found in PATH")
		}
		switch runtime.GOOS {
		case "darwin":
			return buildFFmpegDarwinSpec(device), nil
		case "windows":
			return buildFFmpegWindowsSpec(device), nil
		default:
			return buildFFmpegSpec(device), nil
		}
	default:
		return nil, fmt.Errorf("unsupported recorder %q (use auto, pw-record, parec, arecord, sox, or ffmpeg)", preference)
	}
}

func buildPWRecordSpec(device string) *recorderSpec {
	args := []string{
		"--media-category", "Capture",
		"--rate", "24000",
		"--channels", "1",
		"--format", "s16",
		"--raw",
	}
	if strings.TrimSpace(device) != "" {
		args = append(args, "--target", strings.TrimSpace(device))
	}
	args = append(args, "-")
	return &recorderSpec{name: "pw-record", args: args}
}

func buildParecSpec(device string) *recorderSpec {
	source := strings.TrimSpace(device)
	if source == "" {
		source = "@DEFAULT_SOURCE@"
	}
	return &recorderSpec{
		name: "parec",
		args: []string{
			"--raw",
			"--rate=24000",
			"--format=s16le",
			"--channels=1",
			"--device=" + source,
		},
	}
}

func buildARecordSpec(device string) *recorderSpec {
	args := []string{"-q", "-f", "S16_LE", "-c", "1", "-r", "24000", "-t", "raw"}
	if strings.TrimSpace(device) != "" {
		args = append(args, "-D", strings.TrimSpace(device))
	}
	return &recorderSpec{name: "arecord", args: args}
}

func buildFFmpegSpec(device string) *recorderSpec {
	source := strings.TrimSpace(device)
	if source == "" {
		source = "default"
	}
	return &recorderSpec{
		name: "ffmpeg",
		args: []string{
			"-hide_banner", "-loglevel", "error",
			"-f", "pulse",
			"-i", source,
			"-ac", "1",
			"-ar", "24000",
			"-f", "s16le",
			"-acodec", "pcm_s16le",
			"-",
		},
	}
}

func buildSoxDarwinSpec(device string) *recorderSpec {
	args := []string{"-d", "-t", "raw", "-r", "24000", "-c", "1", "-b", "16", "-e", "signed-integer", "-"}
	return &recorderSpec{name: "sox", args: args}
}

func buildFFmpegDarwinSpec(device string) *recorderSpec {
	source := strings.TrimSpace(device)
	if source == "" {
		source = ":default"
	}
	return &recorderSpec{
		name: "ffmpeg",
		args: []string{
			"-hide_banner", "-loglevel", "error",
			"-f", "avfoundation",
			"-i", source,
			"-ac", "1",
			"-ar", "24000",
			"-f", "s16le",
			"-acodec", "pcm_s16le",
			"-",
		},
	}
}

func buildFFmpegWindowsSpec(device string) *recorderSpec {
	source := strings.TrimSpace(device)
	if source == "" {
		source = "Microphone"
	}
	return &recorderSpec{
		name: "ffmpeg",
		args: []string{
			"-hide_banner", "-loglevel", "error",
			"-f", "dshow",
			"-i", "audio=" + source,
			"-ac", "1",
			"-ar", "24000",
			"-f", "s16le",
			"-acodec", "pcm_s16le",
			"-",
		},
	}
}

func hasUsableARecordDevice() bool {
	out, err := runCommandOutput("arecord", "-L")
	if err != nil {
		return false
	}

	for _, line := range strings.Split(out, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			continue
		}
		if trimmed != "null" {
			return true
		}
	}
	return false
}

func streamAudioChunks(ctx context.Context, ws *websocket.Conn, r io.Reader) (streamStats, error) {
	buf := make([]byte, audioChunkBytes)
	stats := streamStats{}

	for {
		n, err := r.Read(buf)
		if n > 0 {
			stats.audioBytesSent += n
			event := map[string]any{
				"type":  "input_audio_buffer.append",
				"audio": base64.StdEncoding.EncodeToString(buf[:n]),
			}
			if sendErr := websocket.JSON.Send(ws, event); sendErr != nil {
				return stats, fmt.Errorf("send realtime audio chunk: %w", sendErr)
			}
		}

		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(ctx.Err(), context.Canceled) {
				if stats.audioBytesSent >= minCommitAudioBytes {
					_ = websocket.JSON.Send(ws, map[string]any{"type": "input_audio_buffer.commit"})
				}
				return stats, ctx.Err()
			}
			return stats, fmt.Errorf("read microphone audio: %w", err)
		}
	}
}

func readRealtimeTranscriptionEvents(ctx context.Context, ws *websocket.Conn, writer *transcriptWriter) error {
	states := map[string]*transcriptState{}

	for {
		var event realtimeEvent
		if err := websocket.JSON.Receive(ws, &event); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return err
		}

		switch event.Type {
		case "conversation.item.input_audio_transcription.delta":
			state := states[event.ItemID]
			if state == nil {
				state = &transcriptState{}
				states[event.ItemID] = state
			}
			state.printed = true
			state.buf.WriteString(event.Delta)
			fmt.Print(event.Delta)
		case "conversation.item.input_audio_transcription.completed":
			state := states[event.ItemID]
			switch {
			case state == nil || !state.printed:
				if strings.TrimSpace(event.Transcript) != "" {
					fmt.Print(event.Transcript)
				}
			case !strings.HasSuffix(state.buf.String(), "\n"):
				// The transcript was streamed incrementally already; just terminate the line below.
			}
			fmt.Println()
			if err := writer.WriteTurn(event.Transcript); err != nil {
				return err
			}
			delete(states, event.ItemID)
		case "error":
			if event.Error != nil {
				if event.Error.Code == "input_audio_buffer_commit_empty" {
					return nil
				}
				return fmt.Errorf("realtime API error: %s", formatRealtimeError(event.Error))
			}
			raw, _ := json.Marshal(event)
			return fmt.Errorf("realtime API error: %s", string(raw))
		}
	}
}

func formatRealtimeError(errMsg *realtimeErrMsg) string {
	if errMsg == nil {
		return "unknown error"
	}
	parts := []string{}
	if errMsg.Type != "" {
		parts = append(parts, errMsg.Type)
	}
	if errMsg.Code != "" {
		parts = append(parts, errMsg.Code)
	}
	if errMsg.Message != "" {
		parts = append(parts, errMsg.Message)
	}
	if len(parts) == 0 {
		return "unknown error"
	}
	return strings.Join(parts, ": ")
}

func isClosedConnErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "use of closed network connection") || strings.Contains(msg, "connection reset by peer")
}

func millisForAudioBytes(n int) float64 {
	// 24kHz * 16-bit mono = 48,000 bytes per second.
	return float64(n) / 48.0
}

func newTranscriptWriter(path string) (*transcriptWriter, error) {
	if strings.TrimSpace(path) == "" {
		return &transcriptWriter{}, nil
	}
	if err := ensureParentDir(path); err != nil {
		return nil, err
	}
	file, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("create transcript output file: %w", err)
	}
	return &transcriptWriter{file: file}, nil
}

func (w *transcriptWriter) WriteTurn(text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	w.turns++
	if w.file == nil {
		return nil
	}
	if _, err := fmt.Fprintln(w.file, text); err != nil {
		return fmt.Errorf("write transcript output: %w", err)
	}
	return nil
}

func (w *transcriptWriter) Turns() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.turns
}

func (w *transcriptWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	return w.file.Close()
}
