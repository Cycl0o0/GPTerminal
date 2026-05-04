package ai

import (
	"context"
	"errors"
	"io"
	"sort"

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
	return p.client.CreateChatCompletion(ctx, req)
}

func (p *OpenAIProvider) CreateChatCompletionStream(ctx context.Context, req openai.ChatCompletionRequest) (ChatStream, error) {
	stream, err := p.client.CreateChatCompletionStream(ctx, req)
	if err != nil {
		return nil, err
	}
	return &openaiStream{stream: stream}, nil
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
