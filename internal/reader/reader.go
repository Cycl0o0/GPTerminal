package reader

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cycl0o0/GPTerminal/internal/ai"
	"github.com/cycl0o0/GPTerminal/internal/system"
	openai "github.com/sashabaranov/go-openai"
)

const maxTextChars = 100_000

var imageExts = map[string]string{
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".gif":  "image/gif",
	".webp": "image/webp",
}

// FileKind describes the detected file type.
type FileKind string

const (
	KindText   FileKind = "text"
	KindImage  FileKind = "image"
	KindPDF    FileKind = "pdf"
	KindBinary FileKind = "binary"
)

// IsImage returns true if the file extension is a supported image format.
func IsImage(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	_, ok := imageExts[ext]
	return ok
}

// DetectKind returns the file kind based on extension and content.
func DetectKind(path string) FileKind {
	ext := strings.ToLower(filepath.Ext(path))
	if _, ok := imageExts[ext]; ok {
		return KindImage
	}
	if ext == ".pdf" {
		return KindPDF
	}

	// Check first 8KB for null bytes to detect binary
	f, err := os.Open(path)
	if err != nil {
		return KindText
	}
	defer f.Close()

	buf := make([]byte, 8192)
	n, _ := f.Read(buf)
	if bytes.ContainsRune(buf[:n], 0) {
		return KindBinary
	}
	return KindText
}

// ReadFile reads and analyzes a file with AI. Detects type automatically.
func ReadFile(ctx context.Context, path, question string) (string, error) {
	client, err := ai.NewClient()
	if err != nil {
		return "", err
	}

	sysInfo := system.Detect()
	sysPrompt := ai.ReadSystemPrompt(sysInfo.ContextBlock())

	switch DetectKind(path) {
	case KindImage:
		return readImage(ctx, client, sysPrompt, path, question)
	case KindPDF:
		return readPDF(ctx, client, sysPrompt, path, question)
	case KindBinary:
		return "", fmt.Errorf("unsupported binary file format: %s\nSupported binary formats: PDF, PNG, JPG, GIF, WEBP", filepath.Ext(path))
	default:
		return readText(ctx, client, sysPrompt, path, question)
	}
}

func readText(ctx context.Context, client *ai.Client, sysPrompt, path, question string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}

	content := string(data)
	if len(content) > maxTextChars {
		content = content[:maxTextChars] + "\n... (truncated)"
	}

	filename := filepath.Base(path)
	userMsg := fmt.Sprintf("File: %s\n\n```\n%s\n```\n\nQuestion: %s", filename, content, question)

	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: sysPrompt},
		{Role: openai.ChatMessageRoleUser, Content: userMsg},
	}

	return client.Complete(ctx, messages)
}

func readImage(ctx context.Context, client *ai.Client, sysPrompt, path, question string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read image: %w", err)
	}

	ext := strings.ToLower(filepath.Ext(path))
	mimeType := imageExts[ext]
	b64 := base64.StdEncoding.EncodeToString(data)

	prompt := fmt.Sprintf("File: %s\n\n%s", filepath.Base(path), question)
	return client.CompleteVision(ctx, sysPrompt, prompt, b64, mimeType)
}

func readPDF(ctx context.Context, client *ai.Client, sysPrompt, path, question string) (string, error) {
	content, err := extractPDFText(path)
	if err != nil {
		return "", fmt.Errorf("extract PDF text: %w\nInstall poppler-utils: sudo apt install poppler-utils (or brew install poppler)", err)
	}

	if strings.TrimSpace(content) == "" {
		return "", fmt.Errorf("PDF appears to contain no extractable text (may be scanned/image-only)")
	}

	if len(content) > maxTextChars {
		content = content[:maxTextChars] + "\n... (truncated)"
	}

	filename := filepath.Base(path)
	userMsg := fmt.Sprintf("File: %s (PDF document)\n\n```\n%s\n```\n\nQuestion: %s", filename, content, question)

	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: sysPrompt},
		{Role: openai.ChatMessageRoleUser, Content: userMsg},
	}

	return client.Complete(ctx, messages)
}

func extractPDFText(path string) (string, error) {
	// Try pdftotext (poppler-utils)
	if bin, err := exec.LookPath("pdftotext"); err == nil {
		out, err := exec.Command(bin, "-layout", path, "-").Output()
		if err == nil {
			return string(out), nil
		}
	}

	// Try mutool (mupdf-tools)
	if bin, err := exec.LookPath("mutool"); err == nil {
		out, err := exec.Command(bin, "draw", "-F", "text", path).Output()
		if err == nil {
			return string(out), nil
		}
	}

	return "", fmt.Errorf("no PDF text extractor found (tried pdftotext, mutool)")
}
