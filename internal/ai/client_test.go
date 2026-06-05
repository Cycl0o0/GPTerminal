package ai_test

import (
	"context"
	"errors"
	"testing"

	"github.com/cycl0o0/GPTerminal/internal/ai"
	"github.com/cycl0o0/GPTerminal/internal/ai/aifake"
	openai "github.com/sashabaranov/go-openai"
)

func TestNewClientWithProvider_Complete(t *testing.T) {
	fp := &aifake.Provider{Responses: []string{"hello from fake"}}
	c := ai.NewClientWithProvider(fp)

	if c.ProviderName() != "fake" {
		t.Fatalf("ProviderName = %q, want fake", c.ProviderName())
	}

	got, err := c.Complete(context.Background(), []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleUser, Content: "hi"},
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if got != "hello from fake" {
		t.Fatalf("Complete = %q, want %q", got, "hello from fake")
	}
	if len(fp.Calls) != 1 {
		t.Fatalf("recorded %d calls, want 1", len(fp.Calls))
	}
}

func TestNewClientWithProvider_PropagatesError(t *testing.T) {
	sentinel := errors.New("boom")
	c := ai.NewClientWithProvider(&aifake.Provider{Err: sentinel})

	_, err := c.Complete(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("error %v does not wrap sentinel", err)
	}
}
