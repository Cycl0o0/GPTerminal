// Package aifake provides an in-memory ai.Provider implementation for tests.
// It performs no network access; responses are scripted by the caller.
package aifake

import (
	"context"
	"fmt"

	"github.com/cycl0o0/GPTerminal/internal/ai"
	openai "github.com/sashabaranov/go-openai"
)

// Provider is a scriptable, deterministic ai.Provider for tests.
type Provider struct {
	// Responses are returned in order, one per CreateChatCompletion call.
	Responses []string
	// Err, when non-nil, is returned instead of a response.
	Err error
	// Models is returned by ListModels.
	Models []string

	// Calls records every request the code under test sent to the provider.
	Calls []openai.ChatCompletionRequest

	idx int
}

var _ ai.Provider = (*Provider)(nil)

func (p *Provider) Name() string { return "fake" }

func (p *Provider) CreateChatCompletion(_ context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	p.Calls = append(p.Calls, req)
	if p.Err != nil {
		return openai.ChatCompletionResponse{}, p.Err
	}
	content := ""
	if p.idx < len(p.Responses) {
		content = p.Responses[p.idx]
		p.idx++
	}
	return openai.ChatCompletionResponse{
		Choices: []openai.ChatCompletionChoice{
			{Message: openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: content}},
		},
	}, nil
}

func (p *Provider) CreateChatCompletionStream(_ context.Context, req openai.ChatCompletionRequest) (ai.ChatStream, error) {
	p.Calls = append(p.Calls, req)
	if p.Err != nil {
		return nil, p.Err
	}
	content := ""
	if p.idx < len(p.Responses) {
		content = p.Responses[p.idx]
		p.idx++
	}
	return &stream{content: content}, nil
}

func (p *Provider) ListModels(_ context.Context) ([]string, error) {
	if p.Err != nil {
		return nil, p.Err
	}
	return p.Models, nil
}

// LastUserContent returns the content of the most recent user message sent to
// the provider — handy for asserting on what context was forwarded to the LLM.
func (p *Provider) LastUserContent() (string, error) {
	if len(p.Calls) == 0 {
		return "", fmt.Errorf("no calls recorded")
	}
	msgs := p.Calls[len(p.Calls)-1].Messages
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == openai.ChatMessageRoleUser {
			return msgs[i].Content, nil
		}
	}
	return "", fmt.Errorf("no user message in last call")
}

type stream struct {
	content string
	done    bool
}

func (s *stream) Recv() (ai.ChatStreamEvent, error) {
	if s.done {
		return ai.ChatStreamEvent{}, nil
	}
	s.done = true
	return ai.ChatStreamEvent{Content: s.content}, nil
}

func (s *stream) Close() {}
