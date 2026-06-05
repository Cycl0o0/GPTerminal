package ai

import (
	"context"
	"os"

	"github.com/cycl0o0/GPTerminal/internal/redact"
	openai "github.com/sashabaranov/go-openai"
)

// redactionEnabled gates outbound secret redaction. It is ON by default (the
// safe default required by INSTRUCTIONS.md §3) and can only be disabled by an
// explicit opt-out, never by the LLM.
func redactionEnabled() bool {
	switch os.Getenv("GPTERMINAL_REDACT") {
	case "0", "false", "off", "no":
		return false
	}
	return true
}

// redactMessages returns a copy of msgs with secrets masked in every text
// field. This is the single egress chokepoint: nothing reaches a provider
// without passing through here (INSTRUCTIONS.md §5: redact before any LLM send).
func redactMessages(msgs []openai.ChatCompletionMessage) []openai.ChatCompletionMessage {
	if !redactionEnabled() || len(msgs) == 0 {
		return msgs
	}
	out := make([]openai.ChatCompletionMessage, len(msgs))
	for i, m := range msgs {
		if m.Content != "" {
			m.Content = redact.String(m.Content)
		}
		for j := range m.MultiContent {
			if m.MultiContent[j].Type == openai.ChatMessagePartTypeText {
				m.MultiContent[j].Text = redact.String(m.MultiContent[j].Text)
			}
		}
		out[i] = m
	}
	return out
}

// sendChat redacts then forwards a completion request to the provider.
func (c *Client) sendChat(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	req.Messages = redactMessages(req.Messages)
	return c.provider.CreateChatCompletion(ctx, req)
}

// sendChatStream redacts then forwards a streaming request to the provider.
func (c *Client) sendChatStream(ctx context.Context, req openai.ChatCompletionRequest) (ChatStream, error) {
	req.Messages = redactMessages(req.Messages)
	return c.provider.CreateChatCompletionStream(ctx, req)
}
