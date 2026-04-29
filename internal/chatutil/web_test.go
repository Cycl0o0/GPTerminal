package chatutil

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHtmlToText(t *testing.T) {
	input := `<html><body>
		<script>var x = 1;</script>
		<style>body { color: red; }</style>
		<h1>Hello World</h1>
		<p>This is a <b>test</b> paragraph.</p>
		<nav>Skip this</nav>
	</body></html>`

	text, err := htmlToText(strings.NewReader(input))
	if err != nil {
		t.Fatalf("htmlToText error: %v", err)
	}
	if !strings.Contains(text, "Hello World") {
		t.Error("expected 'Hello World' in output")
	}
	if !strings.Contains(text, "test") {
		t.Error("expected 'test' in output")
	}
	if strings.Contains(text, "var x = 1") {
		t.Error("script content should be stripped")
	}
	if strings.Contains(text, "color: red") {
		t.Error("style content should be stripped")
	}
	if strings.Contains(text, "Skip this") {
		t.Error("nav content should be stripped")
	}
}

func TestFetchURL(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("<html><body><p>Hello from test server</p></body></html>"))
	}))
	defer ts.Close()

	out, err := fetchURL(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("fetchURL error: %v", err)
	}
	if !strings.Contains(out, "Hello from test server") {
		t.Errorf("expected test content, got: %s", out)
	}
	if !strings.Contains(out, "URL:") {
		t.Error("expected URL prefix in output")
	}
}

func TestFetchURLPlainText(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("plain text response"))
	}))
	defer ts.Close()

	out, err := fetchURL(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("fetchURL error: %v", err)
	}
	if !strings.Contains(out, "plain text response") {
		t.Errorf("expected plain text, got: %s", out)
	}
}

func TestFetchURLRejectsNonHTTP(t *testing.T) {
	_, err := fetchURL(context.Background(), "ftp://example.com/file")
	if err == nil {
		t.Fatal("expected error for ftp scheme")
	}
}

func TestFetchURLEmpty(t *testing.T) {
	_, err := fetchURL(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty URL")
	}
}

func TestWebSearchEmpty(t *testing.T) {
	_, err := webSearch(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty query")
	}
}

func TestParseDuckDuckGoHTML(t *testing.T) {
	mockHTML := `<html><body>
		<div class="result">
			<a class="result__a" href="/l/?uddg=https%3A%2F%2Fexample.com">Example Site</a>
			<a class="result__snippet">This is the snippet for example.</a>
		</div>
		<div class="result">
			<a class="result__a" href="/l/?uddg=https%3A%2F%2Fother.com">Other Site</a>
			<a class="result__snippet">Another snippet here.</a>
		</div>
	</body></html>`

	results, err := parseDuckDuckGoHTML(strings.NewReader(mockHTML))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Title != "Example Site" {
		t.Errorf("expected 'Example Site', got %q", results[0].Title)
	}
	if results[0].URL != "https://example.com" {
		t.Errorf("expected 'https://example.com', got %q", results[0].URL)
	}
	if results[0].Snippet != "This is the snippet for example." {
		t.Errorf("unexpected snippet: %q", results[0].Snippet)
	}
}
