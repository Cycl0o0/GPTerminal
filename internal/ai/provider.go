package ai

import (
	"context"
	"io"

	openai "github.com/sashabaranov/go-openai"
)

type ChatStream interface {
	Recv() (ChatStreamEvent, error)
	Close()
}

type ChatStreamEvent struct {
	Content          string
	ReasoningContent string
	ToolCalls        []openai.ToolCall
	Usage            *openai.Usage
}

type Provider interface {
	Name() string
	CreateChatCompletion(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error)
	CreateChatCompletionStream(ctx context.Context, req openai.ChatCompletionRequest) (ChatStream, error)
	ListModels(ctx context.Context) ([]string, error)
}

type ImageProvider interface {
	CreateImage(ctx context.Context, req openai.ImageRequest) (openai.ImageResponse, error)
}

type SpeechProvider interface {
	CreateTranscription(ctx context.Context, req openai.AudioRequest) (openai.AudioResponse, error)
	CreateTranslation(ctx context.Context, req openai.AudioRequest) (openai.AudioResponse, error)
	CreateSpeech(ctx context.Context, req openai.CreateSpeechRequest) (io.ReadCloser, error)
}
