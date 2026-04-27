package reader

import (
	"strings"
	"testing"
)

func TestBuildTextUserMessageIncludesSourceAndQuestion(t *testing.T) {
	msg := buildTextUserMessage("stdin", "hello\nworld", "summarize it")

	if !strings.Contains(msg, "File: stdin") {
		t.Fatalf("expected source name in message, got %q", msg)
	}
	if !strings.Contains(msg, "Question: summarize it") {
		t.Fatalf("expected question in message, got %q", msg)
	}
}

func TestBuildTextUserMessageDefaultsQuestion(t *testing.T) {
	msg := buildTextUserMessage("stdin", "hello", "")

	if !strings.Contains(msg, "Question: Summarize and analyze this content.") {
		t.Fatalf("expected default question, got %q", msg)
	}
}

func TestTruncateTextContent(t *testing.T) {
	input := strings.Repeat("a", maxTextChars+5)
	got := truncateTextContent(input)

	if len(got) <= maxTextChars {
		t.Fatalf("expected truncation marker to be appended, got length %d", len(got))
	}
	if !strings.HasSuffix(got, "\n... (truncated)") {
		t.Fatalf("expected truncation marker, got %q", got[len(got)-20:])
	}
}

func TestIsURL(t *testing.T) {
	if !IsURL("https://example.com") {
		t.Fatal("expected https URL to be detected")
	}
	if IsURL("/tmp/file.txt") {
		t.Fatal("did not expect local path to be treated as URL")
	}
}

func TestExtractHTMLText(t *testing.T) {
	text := extractHTMLText("<html><body><h1>Hello</h1><p>world</p></body></html>")
	if !strings.Contains(text, "Hello") || !strings.Contains(text, "world") {
		t.Fatalf("expected extracted HTML text, got %q", text)
	}
}
