package ai

import (
	"context"
	"errors"
	"io"
	"sort"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

type OpenAIProvider struct {
	client *openai.Client
}

func NewOpenAIProvider(apiKey, baseURL string) *OpenAIProvider {
	cfg := openai.DefaultConfig(apiKey)
	cfg.BaseURL = baseURL
	return &OpenAIProvider{client: openai.NewClientWithConfig(cfg)}
}

func (p *OpenAIProvider) Name() string { return "openai" }

func (p *OpenAIProvider) CreateChatCompletion(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	return p.client.CreateChatCompletion(ctx, normalizeReasoningRequest(req))
}

func (p *OpenAIProvider) CreateChatCompletionStream(ctx context.Context, req openai.ChatCompletionRequest) (ChatStream, error) {
	stream, err := p.client.CreateChatCompletionStream(ctx, normalizeReasoningRequest(req))
	if err != nil {
		return nil, err
	}
	return &openaiStream{stream: stream}, nil
}

// isReasoningModel reports whether a model is an OpenAI reasoning model, which
// the go-openai client-side validator holds to stricter request rules.
func isReasoningModel(model string) bool {
	return strings.HasPrefix(model, "o1") ||
		strings.HasPrefix(model, "o3") ||
		strings.HasPrefix(model, "o4") ||
		strings.HasPrefix(model, "gpt-5")
}

// normalizeReasoningRequest adapts a request for OpenAI reasoning models. The
// go-openai ReasoningValidator rejects MaxTokens (must be MaxCompletionTokens)
// and any Temperature other than 0 or 1 for o1/o3/o4/gpt-5 models — the exact
// combination GPTerminal builds by default — so without this, those models are
// unusable. For non-reasoning models the request is returned unchanged.
func normalizeReasoningRequest(req openai.ChatCompletionRequest) openai.ChatCompletionRequest {
	if !isReasoningModel(req.Model) {
		// reasoning_effort only applies to reasoning models; drop it elsewhere
		// to avoid surprising 400s from stricter servers.
		req.ReasoningEffort = ""
		return req
	}
	if req.MaxTokens > 0 {
		if req.MaxCompletionTokens == 0 {
			req.MaxCompletionTokens = req.MaxTokens
		}
		req.MaxTokens = 0
	}
	// Temperature must be 0 or 1; our default (0.7) is rejected, so clear it.
	if req.Temperature != 0 && req.Temperature != 1 {
		req.Temperature = 0
	}
	// OpenAI has no "none"/"max"; clamp to the supported set.
	switch req.ReasoningEffort {
	case "minimal", "low", "medium", "high":
		// supported as-is
	case "max":
		req.ReasoningEffort = "high"
	default:
		req.ReasoningEffort = ""
	}
	return req
}

func (p *OpenAIProvider) ListModels(ctx context.Context) ([]string, error) {
	list, err := p.client.ListModels(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(list.Models))
	for i, m := range list.Models {
		ids[i] = m.ID
	}
	sort.Strings(ids)
	return ids, nil
}

func (p *OpenAIProvider) CreateImage(ctx context.Context, req openai.ImageRequest) (openai.ImageResponse, error) {
	return p.client.CreateImage(ctx, req)
}

func (p *OpenAIProvider) CreateTranscription(ctx context.Context, req openai.AudioRequest) (openai.AudioResponse, error) {
	return p.client.CreateTranscription(ctx, req)
}

func (p *OpenAIProvider) CreateTranslation(ctx context.Context, req openai.AudioRequest) (openai.AudioResponse, error) {
	return p.client.CreateTranslation(ctx, req)
}

func (p *OpenAIProvider) CreateSpeech(ctx context.Context, req openai.CreateSpeechRequest) (io.ReadCloser, error) {
	return p.client.CreateSpeech(ctx, req)
}

type openaiStream struct {
	stream *openai.ChatCompletionStream
}

func (s *openaiStream) Recv() (ChatStreamEvent, error) {
	resp, err := s.stream.Recv()
	if errors.Is(err, io.EOF) {
		return ChatStreamEvent{}, io.EOF
	}
	if err != nil {
		return ChatStreamEvent{}, err
	}
	evt := ChatStreamEvent{
		Usage: resp.Usage,
	}
	if len(resp.Choices) > 0 {
		evt.Content = resp.Choices[0].Delta.Content
		evt.ReasoningContent = resp.Choices[0].Delta.ReasoningContent
		evt.ToolCalls = resp.Choices[0].Delta.ToolCalls
	}
	return evt, nil
}

func (s *openaiStream) Close() {
	s.stream.Close()
}
