package reader

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cycl0o0/GPTerminal/internal/ai"
	"github.com/cycl0o0/GPTerminal/internal/system"
	openai "github.com/sashabaranov/go-openai"
	"golang.org/x/net/html"
)

const (
	maxTextChars   = 100_000
	maxRemoteBytes = 25 * 1024 * 1024
)

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

func IsURL(input string) bool {
	input = strings.TrimSpace(strings.ToLower(input))
	return strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://")
}

// ReadFile reads and analyzes a file with AI. Detects type automatically.
func ReadFile(ctx context.Context, path, question string) (string, error) {
	client, sysPrompt, err := newClientAndPrompt()
	if err != nil {
		return "", err
	}

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

// ReadTextInput analyzes plain text content such as piped stdin.
func ReadTextInput(ctx context.Context, name, content, question string) (string, error) {
	client, sysPrompt, err := newClientAndPrompt()
	if err != nil {
		return "", err
	}
	return readTextContent(ctx, client, sysPrompt, name, content, question)
}

// ReadURL fetches a remote URL and analyzes it similarly to a local file.
func ReadURL(ctx context.Context, rawURL, question string) (string, error) {
	client, sysPrompt, err := newClientAndPrompt()
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("fetch URL: unexpected HTTP status %s", resp.Status)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxRemoteBytes+1))
	if err != nil {
		return "", fmt.Errorf("read URL body: %w", err)
	}
	if len(data) > maxRemoteBytes {
		return "", fmt.Errorf("remote content exceeds %d MB limit", maxRemoteBytes/(1024*1024))
	}

	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	name := displayNameForURL(rawURL)
	switch detectRemoteKind(rawURL, contentType, data) {
	case KindImage:
		return readImageData(ctx, client, sysPrompt, name, data, detectRemoteImageMIME(rawURL, contentType), question)
	case KindPDF:
		return readPDFData(ctx, client, sysPrompt, name, data, question)
	case KindBinary:
		return "", fmt.Errorf("unsupported remote binary format")
	default:
		text := string(data)
		if strings.Contains(contentType, "text/html") {
			text = extractHTMLText(text)
		}
		return readTextContent(ctx, client, sysPrompt, name, text, question)
	}
}

func newClientAndPrompt() (*ai.Client, string, error) {
	client, err := ai.NewClient()
	if err != nil {
		return nil, "", err
	}

	sysInfo := system.Detect()
	return client, ai.ReadSystemPrompt(sysInfo.ContextBlock()), nil
}

func readText(ctx context.Context, client *ai.Client, sysPrompt, path, question string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}

	return readTextContent(ctx, client, sysPrompt, filepath.Base(path), string(data), question)
}

func readImage(ctx context.Context, client *ai.Client, sysPrompt, path, question string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read image: %w", err)
	}

	ext := strings.ToLower(filepath.Ext(path))
	mimeType := imageExts[ext]
	return readImageData(ctx, client, sysPrompt, filepath.Base(path), data, mimeType, question)
}

func readPDF(ctx context.Context, client *ai.Client, sysPrompt, path, question string) (string, error) {
	content, err := extractPDFText(path)
	if err != nil {
		return "", fmt.Errorf("extract PDF text: %w\nInstall poppler-utils: sudo apt install poppler-utils (or brew install poppler)", err)
	}

	if strings.TrimSpace(content) == "" {
		return "", fmt.Errorf("PDF appears to contain no extractable text (may be scanned/image-only)")
	}

	filename := filepath.Base(path)
	userMsg := buildTextUserMessage(filename+" (PDF document)", content, question)

	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: sysPrompt},
		{Role: openai.ChatMessageRoleUser, Content: userMsg},
	}

	return client.Complete(ctx, messages)
}

func readImageData(ctx context.Context, client *ai.Client, sysPrompt, name string, data []byte, mimeType, question string) (string, error) {
	b64 := base64.StdEncoding.EncodeToString(data)
	prompt := fmt.Sprintf("File: %s\n\n%s", name, question)
	return client.CompleteVision(ctx, sysPrompt, prompt, b64, mimeType)
}

func readPDFData(ctx context.Context, client *ai.Client, sysPrompt, name string, data []byte, question string) (string, error) {
	tmpFile, err := os.CreateTemp("", "gpterminal-read-*.pdf")
	if err != nil {
		return "", fmt.Errorf("create temp PDF: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := tmpFile.Write(data); err != nil {
		return "", fmt.Errorf("write temp PDF: %w", err)
	}
	return readPDF(ctx, client, sysPrompt, tmpFile.Name(), question)
}

func readTextContent(ctx context.Context, client *ai.Client, sysPrompt, name, content, question string) (string, error) {
	userMsg := buildTextUserMessage(name, content, question)

	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: sysPrompt},
		{Role: openai.ChatMessageRoleUser, Content: userMsg},
	}

	return client.Complete(ctx, messages)
}

func buildTextUserMessage(name, content, question string) string {
	content = truncateTextContent(content)
	question = strings.TrimSpace(question)
	if question == "" {
		question = "Summarize and analyze this content."
	}
	return fmt.Sprintf("File: %s\n\n```\n%s\n```\n\nQuestion: %s", name, content, question)
}

func truncateTextContent(content string) string {
	if len(content) > maxTextChars {
		return content[:maxTextChars] + "\n... (truncated)"
	}
	return content
}

func detectRemoteKind(rawURL, contentType string, data []byte) FileKind {
	contentType = strings.ToLower(contentType)
	if strings.HasPrefix(contentType, "image/") || IsImage(rawURL) {
		return KindImage
	}
	if strings.Contains(contentType, "application/pdf") || strings.HasSuffix(strings.ToLower(rawURL), ".pdf") {
		return KindPDF
	}
	if strings.HasPrefix(contentType, "text/") || strings.Contains(contentType, "json") || strings.Contains(contentType, "xml") || strings.Contains(contentType, "html") {
		return KindText
	}
	if bytes.IndexByte(data, 0) >= 0 {
		return KindBinary
	}
	return KindText
}

func detectRemoteImageMIME(rawURL, contentType string) string {
	if strings.HasPrefix(contentType, "image/") {
		if idx := strings.Index(contentType, ";"); idx >= 0 {
			return contentType[:idx]
		}
		return contentType
	}

	ext := strings.ToLower(filepath.Ext(rawURL))
	if mime, ok := imageExts[ext]; ok {
		return mime
	}
	return "image/png"
}

func displayNameForURL(rawURL string) string {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return "remote content"
	}
	return trimmed
}

func extractHTMLText(source string) string {
	doc, err := html.Parse(strings.NewReader(source))
	if err != nil {
		return source
	}

	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			text := strings.TrimSpace(n.Data)
			if text != "" {
				if b.Len() > 0 {
					b.WriteByte('\n')
				}
				b.WriteString(text)
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	if strings.TrimSpace(b.String()) == "" {
		return source
	}
	return b.String()
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
