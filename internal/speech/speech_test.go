package speech

import (
	"os"
	"path/filepath"
	"testing"

	openai "github.com/sashabaranov/go-openai"
)

func TestParseTranscriptionFormat(t *testing.T) {
	got, err := ParseTranscriptionFormat("verbose_json")
	if err != nil {
		t.Fatalf("ParseTranscriptionFormat returned error: %v", err)
	}
	if got != openai.AudioResponseFormatVerboseJSON {
		t.Fatalf("unexpected format: %q", got)
	}
}

func TestParseSpeechFormat(t *testing.T) {
	got, err := ParseSpeechFormat("wav")
	if err != nil {
		t.Fatalf("ParseSpeechFormat returned error: %v", err)
	}
	if got != openai.SpeechResponseFormatWav {
		t.Fatalf("unexpected format: %q", got)
	}
}

func TestValidateAudioInputFileRejectsUnsupportedFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.ogg")
	if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
		t.Fatalf("write sample: %v", err)
	}

	if err := validateAudioInputFile(path); err == nil {
		t.Fatal("expected unsupported format error, got nil")
	}
}

func TestValidateAudioInputFileRejectsOversizeFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.mp3")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create sample: %v", err)
	}
	if err := f.Truncate(maxAudioUploadBytes + 1); err != nil {
		t.Fatalf("truncate sample: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close sample: %v", err)
	}

	if err := validateAudioInputFile(path); err == nil {
		t.Fatal("expected oversize file error, got nil")
	}
}

func TestDefaultSpeechOutputPath(t *testing.T) {
	if got := defaultSpeechOutputPath(openai.SpeechResponseFormatFlac); got != "speech.flac" {
		t.Fatalf("unexpected default output path: %q", got)
	}
}

func TestBuildRealtimeSessionUpdate(t *testing.T) {
	event := buildRealtimeSessionUpdate("gpt-4o-mini-transcribe", "fr", "terminal")

	if event["type"] != "session.update" {
		t.Fatalf("unexpected event type: %#v", event["type"])
	}

	session, ok := event["session"].(map[string]any)
	if !ok {
		t.Fatalf("missing session payload: %#v", event["session"])
	}
	if session["type"] != "transcription" {
		t.Fatalf("unexpected session type: %#v", session["type"])
	}

	audio := session["audio"].(map[string]any)
	input := audio["input"].(map[string]any)
	format := input["format"].(map[string]any)
	if format["type"] != "audio/pcm" {
		t.Fatalf("unexpected audio format type: %#v", format["type"])
	}
	if format["rate"] != 24000 {
		t.Fatalf("unexpected audio format rate: %#v", format["rate"])
	}
	transcription := input["transcription"].(map[string]any)

	if transcription["model"] != "gpt-4o-mini-transcribe" {
		t.Fatalf("unexpected transcription model: %#v", transcription["model"])
	}
	if transcription["language"] != "fr" {
		t.Fatalf("unexpected language: %#v", transcription["language"])
	}
	if transcription["prompt"] != "terminal" {
		t.Fatalf("unexpected prompt: %#v", transcription["prompt"])
	}
}

func TestMillisForAudioBytes(t *testing.T) {
	if got := millisForAudioBytes(4800); got != 100 {
		t.Fatalf("unexpected duration for 4800 bytes: %v", got)
	}
}

func TestDetectRecorderSelection(t *testing.T) {
	origLookPath := lookPath
	origRunCommandOutput := runCommandOutput
	defer func() {
		lookPath = origLookPath
		runCommandOutput = origRunCommandOutput
	}()

	lookPath = func(name string) (string, error) {
		if name == "pw-record" {
			return "/usr/bin/pw-record", nil
		}
		return "", os.ErrNotExist
	}
	runCommandOutput = func(name string, args ...string) (string, error) {
		return "", os.ErrNotExist
	}

	spec, err := detectRecorder("auto", "")
	if err != nil {
		t.Fatalf("detectRecorder returned error: %v", err)
	}
	if spec.name != "pw-record" {
		t.Fatalf("expected pw-record preference, got %q", spec.name)
	}
}

func TestDetectRecorderSkipsARecordWhenOnlyNullDeviceExists(t *testing.T) {
	origLookPath := lookPath
	origRunCommandOutput := runCommandOutput
	defer func() {
		lookPath = origLookPath
		runCommandOutput = origRunCommandOutput
	}()

	lookPath = func(name string) (string, error) {
		switch name {
		case "arecord", "ffmpeg":
			return "/usr/bin/" + name, nil
		default:
			return "", os.ErrNotExist
		}
	}
	runCommandOutput = func(name string, args ...string) (string, error) {
		return "null\n    Discard all samples\n", nil
	}

	spec, err := detectRecorder("auto", "")
	if err != nil {
		t.Fatalf("detectRecorder returned error: %v", err)
	}
	if spec.name != "ffmpeg" {
		t.Fatalf("expected ffmpeg fallback when arecord has no usable device, got %q", spec.name)
	}
}
