package speech

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cycl0o0/GPTerminal/internal/ai"
	"github.com/cycl0o0/GPTerminal/internal/config"
	openai "github.com/sashabaranov/go-openai"
)

const (
	DefaultTranscriptionModel  = "gpt-4o-mini-transcribe"
	DefaultSpeechModel         = "gpt-4o-mini-tts"
	DefaultSpeechVoice         = "marin"
	DefaultTranscriptionFormat = openai.AudioResponseFormatText
	DefaultSpeechFormat        = openai.SpeechResponseFormatMp3
	maxAudioUploadBytes        = 25 * 1024 * 1024
)

var supportedAudioInputExts = map[string]struct{}{
	".m4a":  {},
	".mp3":  {},
	".mp4":  {},
	".mpeg": {},
	".mpga": {},
	".wav":  {},
	".webm": {},
}

type TranscriptionOptions struct {
	Model              string
	Language           string
	Prompt             string
	Format             openai.AudioResponseFormat
	TranslateToEnglish bool
	OutputPath         string
}

type TranscriptionResult struct {
	Content    string
	OutputPath string
}

type SynthesisOptions struct {
	Model        string
	Voice        string
	Instructions string
	Format       openai.SpeechResponseFormat
	OutputPath   string
	Speed        float64
}

type SynthesisResult struct {
	OutputPath string
	Bytes      int
}

func ParseTranscriptionFormat(value string) (openai.AudioResponseFormat, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "text":
		return openai.AudioResponseFormatText, nil
	case "json":
		return openai.AudioResponseFormatJSON, nil
	case "verbose_json":
		return openai.AudioResponseFormatVerboseJSON, nil
	case "srt":
		return openai.AudioResponseFormatSRT, nil
	case "vtt":
		return openai.AudioResponseFormatVTT, nil
	default:
		return "", fmt.Errorf("unsupported transcription format %q (use text, json, verbose_json, srt, or vtt)", value)
	}
}

func ParseSpeechFormat(value string) (openai.SpeechResponseFormat, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "mp3":
		return openai.SpeechResponseFormatMp3, nil
	case "opus":
		return openai.SpeechResponseFormatOpus, nil
	case "aac":
		return openai.SpeechResponseFormatAac, nil
	case "flac":
		return openai.SpeechResponseFormatFlac, nil
	case "wav":
		return openai.SpeechResponseFormatWav, nil
	case "pcm":
		return openai.SpeechResponseFormatPcm, nil
	default:
		return "", fmt.Errorf("unsupported speech format %q (use mp3, opus, aac, flac, wav, or pcm)", value)
	}
}

func Transcribe(ctx context.Context, filePath string, opts TranscriptionOptions) (*TranscriptionResult, error) {
	if err := validateAudioInputFile(filePath); err != nil {
		return nil, err
	}

	client, err := ai.NewClientWithBaseURL(config.S2TBaseURL())
	if err != nil {
		return nil, err
	}

	req := openai.AudioRequest{
		Model:    opts.Model,
		FilePath: filePath,
		Prompt:   opts.Prompt,
		Format:   opts.Format,
	}
	if !opts.TranslateToEnglish && strings.TrimSpace(opts.Language) != "" {
		req.Language = strings.TrimSpace(opts.Language)
	}

	var resp openai.AudioResponse
	if opts.TranslateToEnglish {
		resp, err = client.CreateTranslation(ctx, req)
	} else {
		resp, err = client.CreateTranscription(ctx, req)
	}
	if err != nil {
		return nil, err
	}

	content, err := formatTranscriptionResponse(resp, opts.Format)
	if err != nil {
		return nil, err
	}

	if outputPath := strings.TrimSpace(opts.OutputPath); outputPath != "" {
		if err := writeTextFile(outputPath, content); err != nil {
			return nil, err
		}
		return &TranscriptionResult{
			Content:    content,
			OutputPath: outputPath,
		}, nil
	}

	return &TranscriptionResult{Content: content}, nil
}

func Synthesize(ctx context.Context, input string, opts SynthesisOptions) (*SynthesisResult, error) {
	if strings.TrimSpace(input) == "" {
		return nil, fmt.Errorf("input text cannot be empty")
	}

	client, err := ai.NewClientWithBaseURL(config.T2SBaseURL())
	if err != nil {
		return nil, err
	}

	data, err := client.CreateSpeech(ctx, openai.CreateSpeechRequest{
		Model:          openai.SpeechModel(opts.Model),
		Input:          input,
		Voice:          openai.SpeechVoice(opts.Voice),
		Instructions:   opts.Instructions,
		ResponseFormat: opts.Format,
		Speed:          opts.Speed,
	})
	if err != nil {
		return nil, err
	}

	outputPath := strings.TrimSpace(opts.OutputPath)
	if outputPath == "" {
		outputPath = defaultSpeechOutputPath(opts.Format)
	}

	if err := writeBinaryFile(outputPath, data); err != nil {
		return nil, err
	}

	return &SynthesisResult{
		OutputPath: outputPath,
		Bytes:      len(data),
	}, nil
}

func validateAudioInputFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat audio file: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("audio input must be a file, not a directory")
	}
	if info.Size() > maxAudioUploadBytes {
		return fmt.Errorf("audio file exceeds OpenAI's 25 MB upload limit: %.1f MB", float64(info.Size())/(1024*1024))
	}

	ext := strings.ToLower(filepath.Ext(path))
	if _, ok := supportedAudioInputExts[ext]; !ok {
		return fmt.Errorf("unsupported audio format %q (supported: %s)", ext, strings.Join(supportedAudioExtensions(), ", "))
	}
	return nil
}

func supportedAudioExtensions() []string {
	exts := make([]string, 0, len(supportedAudioInputExts))
	for ext := range supportedAudioInputExts {
		exts = append(exts, ext)
	}
	sort.Strings(exts)
	return exts
}

func formatTranscriptionResponse(resp openai.AudioResponse, format openai.AudioResponseFormat) (string, error) {
	switch format {
	case openai.AudioResponseFormatJSON, openai.AudioResponseFormatVerboseJSON:
		data, err := json.MarshalIndent(resp, "", "  ")
		if err != nil {
			return "", fmt.Errorf("marshal transcription JSON: %w", err)
		}
		return string(data), nil
	case openai.AudioResponseFormatSRT, openai.AudioResponseFormatVTT:
		return resp.Text, nil
	default:
		return strings.TrimSpace(resp.Text), nil
	}
}

func defaultSpeechOutputPath(format openai.SpeechResponseFormat) string {
	return "speech" + speechFileExtension(format)
}

func speechFileExtension(format openai.SpeechResponseFormat) string {
	switch format {
	case openai.SpeechResponseFormatOpus:
		return ".opus"
	case openai.SpeechResponseFormatAac:
		return ".aac"
	case openai.SpeechResponseFormatFlac:
		return ".flac"
	case openai.SpeechResponseFormatWav:
		return ".wav"
	case openai.SpeechResponseFormatPcm:
		return ".pcm"
	default:
		return ".mp3"
	}
}

func writeTextFile(path, content string) error {
	if err := ensureParentDir(path); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("write output file: %w", err)
	}
	return nil
}

func writeBinaryFile(path string, data []byte) error {
	if err := ensureParentDir(path); err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write audio file: %w", err)
	}
	return nil
}

func ensureParentDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	return nil
}
