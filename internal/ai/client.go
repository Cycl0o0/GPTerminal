package ai

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/cycl0o0/GPTerminal/internal/config"
	gperr "github.com/cycl0o0/GPTerminal/internal/errors"
	"github.com/cycl0o0/GPTerminal/internal/usage"
	openai "github.com/sashabaranov/go-openai"
)

type Client struct {
	provider    Provider
	openaiProv  *OpenAIProvider
}

func NewClient() (*Client, error) {
	return NewClientWithBaseURL(config.APIBaseURL())
}

func NewClientWithBaseURL(baseURL string) (*Client, error) {
	provider := config.ProviderName()

	switch provider {
	case "anthropic":
		key := config.AnthropicAPIKey()
		if key == "" {
			return nil, fmt.Errorf("Anthropic API key not set. Run: gpterminal config set anthropic_api_key <key>\nOr set ANTHROPIC_API_KEY environment variable")
		}
		ap := NewAnthropicProvider(key)
		return &Client{provider: ap}, nil

	case "gemini":
		key := config.GeminiAPIKey()
		if key == "" {
			return nil, fmt.Errorf("Gemini API key not set. Run: gpterminal config set gemini_api_key <key>\nOr set GEMINI_API_KEY environment variable")
		}
		gp := NewGeminiProvider(key)
		return &Client{provider: gp}, nil

	default:
		key := config.APIKey()
		if key == "" && baseURL == config.DefaultBaseURL {
			return nil, fmt.Errorf("API key not set. Run: gpterminal config set-key <key>\nOr set OPENAI_API_KEY environment variable")
		}
		op := NewOpenAIProvider(key, baseURL)
		return &Client{provider: op, openaiProv: op}, nil
	}
}

func (c *Client) ProviderName() string {
	return c.provider.Name()
}

func (c *Client) Complete(ctx context.Context, messages []openai.ChatCompletionMessage) (string, error) {
	if err := usage.Global().CheckBudget(); err != nil {
		return "", err
	}

	model := config.Model()
	resp, err := c.provider.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:       model,
		Messages:    messages,
		Temperature: config.Temperature(),
		MaxTokens:   config.MaxTokens(),
	})
	if err != nil {
		return "", &gperr.APIError{Op: "complete", Message: "API error", Err: err}
	}
	if len(resp.Choices) == 0 {
		return "", &gperr.APIError{Op: "complete", Message: "no response from API"}
	}

	usage.Global().RecordUsage(model, resp.Usage.PromptTokens, resp.Usage.CompletionTokens)
	usage.Global().WarnIfNeeded()

	return resp.Choices[0].Message.Content, nil
}

func (c *Client) CreateChatCompletion(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	if err := usage.Global().CheckBudget(); err != nil {
		return openai.ChatCompletionResponse{}, err
	}

	resp, err := c.provider.CreateChatCompletion(ctx, req)
	if err != nil {
		return openai.ChatCompletionResponse{}, &gperr.APIError{Op: "complete", Message: "API error", Err: err}
	}
	usage.Global().RecordUsage(req.Model, resp.Usage.PromptTokens, resp.Usage.CompletionTokens)
	usage.Global().WarnIfNeeded()
	return resp, nil
}

func (c *Client) CreateChatCompletionStream(ctx context.Context, req openai.ChatCompletionRequest) (ChatStream, error) {
	if err := usage.Global().CheckBudget(); err != nil {
		return nil, err
	}

	stream, err := c.provider.CreateChatCompletionStream(ctx, req)
	if err != nil {
		return nil, &gperr.APIError{Op: "stream", Message: "stream error", Err: err}
	}
	return stream, nil
}

func (c *Client) RecordUsage(model string, usageData *openai.Usage) {
	if usageData == nil {
		return
	}
	usage.Global().RecordUsage(model, usageData.PromptTokens, usageData.CompletionTokens)
	usage.Global().WarnIfNeeded()
}

func (c *Client) StreamComplete(ctx context.Context, messages []openai.ChatCompletionMessage, onChunk func(string)) (string, error) {
	if err := usage.Global().CheckBudget(); err != nil {
		return "", err
	}

	model := config.Model()
	stream, err := c.provider.CreateChatCompletionStream(ctx, openai.ChatCompletionRequest{
		Model:       model,
		Messages:    messages,
		Temperature: config.Temperature(),
		MaxTokens:   config.MaxTokens(),
		Stream:      true,
		StreamOptions: &openai.StreamOptions{
			IncludeUsage: true,
		},
	})
	if err != nil {
		return "", &gperr.APIError{Op: "stream", Message: "stream error", Err: err}
	}
	defer stream.Close()

	var full string
	var streamUsage *openai.Usage
	for {
		evt, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return full, &gperr.APIError{Op: "stream", Message: "recv error", Err: err}
		}
		if evt.Usage != nil {
			streamUsage = evt.Usage
		}
		if evt.Content != "" {
			full += evt.Content
			if onChunk != nil {
				onChunk(evt.Content)
			}
		}
	}

	if streamUsage != nil {
		usage.Global().RecordUsage(model, streamUsage.PromptTokens, streamUsage.CompletionTokens)
		usage.Global().WarnIfNeeded()
	}

	return full, nil
}

func (c *Client) CompleteVision(ctx context.Context, systemPrompt, question, base64Image, mimeType string) (string, error) {
	if err := usage.Global().CheckBudget(); err != nil {
		return "", err
	}

	model := config.Model()
	dataURI := fmt.Sprintf("data:%s;base64,%s", mimeType, base64Image)

	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
		{
			Role: openai.ChatMessageRoleUser,
			MultiContent: []openai.ChatMessagePart{
				{
					Type: openai.ChatMessagePartTypeText,
					Text: question,
				},
				{
					Type: openai.ChatMessagePartTypeImageURL,
					ImageURL: &openai.ChatMessageImageURL{
						URL:    dataURI,
						Detail: openai.ImageURLDetailAuto,
					},
				},
			},
		},
	}

	resp, err := c.provider.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:       model,
		Messages:    messages,
		Temperature: config.Temperature(),
		MaxTokens:   config.MaxTokens(),
	})
	if err != nil {
		return "", &gperr.APIError{Op: "vision", Message: "API error", Err: err}
	}
	if len(resp.Choices) == 0 {
		return "", &gperr.APIError{Op: "vision", Message: "no response from API"}
	}

	usage.Global().RecordUsage(model, resp.Usage.PromptTokens, resp.Usage.CompletionTokens)
	usage.Global().WarnIfNeeded()

	return resp.Choices[0].Message.Content, nil
}

type ImageResult struct {
	B64JSON       string
	RevisedPrompt string
}

func (c *Client) CreateImage(ctx context.Context, prompt, model, size string, n int) ([]ImageResult, error) {
	if c.openaiProv == nil {
		return nil, fmt.Errorf("image generation is only supported with the OpenAI provider")
	}
	if err := usage.Global().CheckBudget(); err != nil {
		return nil, err
	}

	req := openai.ImageRequest{
		Prompt: prompt,
		Model:  model,
		N:      n,
		Size:   size,
	}

	if strings.HasPrefix(model, "gpt-image") {
		req.OutputFormat = openai.CreateImageOutputFormatPNG
	} else {
		req.ResponseFormat = openai.CreateImageResponseFormatB64JSON
	}

	resp, err := c.openaiProv.CreateImage(ctx, req)
	if err != nil {
		return nil, &gperr.APIError{Op: "image", Message: "API error", Err: err}
	}

	usage.Global().RecordImageUsage(model, size, len(resp.Data))
	usage.Global().WarnIfNeeded()

	results := make([]ImageResult, len(resp.Data))
	for i, d := range resp.Data {
		results[i] = ImageResult{
			B64JSON:       d.B64JSON,
			RevisedPrompt: d.RevisedPrompt,
		}
	}
	return results, nil
}

func (c *Client) ListModels(ctx context.Context) ([]string, error) {
	models, err := c.provider.ListModels(ctx)
	if err != nil {
		return nil, &gperr.APIError{Op: "list-models", Message: "API error", Err: err}
	}
	return models, nil
}

func (c *Client) CreateTranscription(ctx context.Context, request openai.AudioRequest) (openai.AudioResponse, error) {
	if c.openaiProv == nil {
		return openai.AudioResponse{}, fmt.Errorf("transcription is only supported with the OpenAI provider")
	}
	if err := usage.Global().CheckBudget(); err != nil {
		return openai.AudioResponse{}, err
	}

	resp, err := c.openaiProv.CreateTranscription(ctx, request)
	if err != nil {
		return openai.AudioResponse{}, &gperr.APIError{Op: "transcription", Message: "API error", Err: err}
	}
	return resp, nil
}

func (c *Client) CreateTranslation(ctx context.Context, request openai.AudioRequest) (openai.AudioResponse, error) {
	if c.openaiProv == nil {
		return openai.AudioResponse{}, fmt.Errorf("translation is only supported with the OpenAI provider")
	}
	if err := usage.Global().CheckBudget(); err != nil {
		return openai.AudioResponse{}, err
	}

	resp, err := c.openaiProv.CreateTranslation(ctx, request)
	if err != nil {
		return openai.AudioResponse{}, &gperr.APIError{Op: "translation", Message: "API error", Err: err}
	}
	return resp, nil
}

func (c *Client) CreateSpeech(ctx context.Context, request openai.CreateSpeechRequest) ([]byte, error) {
	if c.openaiProv == nil {
		return nil, fmt.Errorf("speech synthesis is only supported with the OpenAI provider")
	}
	if err := usage.Global().CheckBudget(); err != nil {
		return nil, err
	}

	resp, err := c.openaiProv.CreateSpeech(ctx, request)
	if err != nil {
		return nil, &gperr.APIError{Op: "speech", Message: "API error", Err: err}
	}
	defer resp.Close()

	data, err := io.ReadAll(resp)
	if err != nil {
		return nil, fmt.Errorf("read speech audio: %w", err)
	}
	return data, nil
}
