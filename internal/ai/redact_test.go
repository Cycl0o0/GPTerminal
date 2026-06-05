package ai

import (
	"context"
	"strings"
	"testing"

	openai "github.com/sashabaranov/go-openai"
)

// recordingProvider captures the request it received so the test can assert on
// what was actually sent to the provider boundary.
type recordingProvider struct {
	got openai.ChatCompletionRequest
}

func (p *recordingProvider) Name() string { return "rec" }
func (p *recordingProvider) CreateChatCompletion(_ context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	p.got = req
	return openai.ChatCompletionResponse{
		Choices: []openai.ChatCompletionChoice{{Message: openai.ChatCompletionMessage{Content: "ok"}}},
	}, nil
}
func (p *recordingProvider) CreateChatCompletionStream(_ context.Context, req openai.ChatCompletionRequest) (ChatStream, error) {
	p.got = req
	return nil, nil
}
func (p *recordingProvider) ListModels(context.Context) ([]string, error) { return nil, nil }

func TestSendChat_RedactsSecretsBeforeProvider(t *testing.T) {
	rp := &recordingProvider{}
	c := NewClientWithProvider(rp)

	_, err := c.sendChat(context.Background(), openai.ChatCompletionRequest{
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleUser, Content: "my key is sk-abcdefghij1234567890ABCD please help"},
		},
	})
	if err != nil {
		t.Fatalf("sendChat: %v", err)
	}
	sent := rp.got.Messages[0].Content
	if strings.Contains(sent, "sk-abcdefghij1234567890ABCD") {
		t.Fatalf("secret reached provider: %q", sent)
	}
	if !strings.Contains(sent, "please help") {
		t.Fatalf("non-secret text was lost: %q", sent)
	}
}

func TestRedactMessages_RespectsOptOut(t *testing.T) {
	t.Setenv("GPTERMINAL_REDACT", "0")
	in := []openai.ChatCompletionMessage{{Content: "sk-abcdefghij1234567890ABCD"}}
	out := redactMessages(in)
	if out[0].Content != in[0].Content {
		t.Fatalf("opt-out should disable redaction, got %q", out[0].Content)
	}
}
