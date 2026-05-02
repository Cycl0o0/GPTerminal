package ai

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/cycl0o0/GPTerminal/internal/config"
	gperr "github.com/cycl0o0/GPTerminal/internal/errors"
	"github.com/cycl0o0/GPTerminal/internal/usage"
	openai "github.com/sashabaranov/go-openai"
)

type Client struct {
	client *openai.Client
}

func NewClient() (*Client, error) {
	key := config.APIKey()
	baseURL := config.APIBaseURL()

	// When using a custom base URL (e.g. Ollama), the API key is optional
	if key == "" && baseURL == config.DefaultBaseURL {
		return nil, fmt.Errorf("OpenAI API key not set. Run: gpterminal config set-key <key>\nOr set OPENAI_API_KEY environment variable")
	}

	cfg := openai.DefaultConfig(key)
	cfg.BaseURL = baseURL
	return &Client{client: openai.NewClientWithConfig(cfg)}, nil
}

func (c *Client) Complete(ctx context.Context, messages []openai.ChatCompletionMessage) (string, error) {
	if err := usage.Global().CheckBudget(); err != nil {
		return "", err
	}

	model := config.Model()
	resp, err := c.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
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

	resp, err := c.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return openai.ChatCompletionResponse{}, &gperr.APIError{Op: "complete", Message: "API error", Err: err}
	}
	usage.Global().RecordUsage(req.Model, resp.Usage.PromptTokens, resp.Usage.CompletionTokens)
	usage.Global().WarnIfNeeded()
	return resp, nil
}

func (c *Client) CreateChatCompletionStream(ctx context.Context, req openai.ChatCompletionRequest) (*openai.ChatCompletionStream, error) {
	if err := usage.Global().CheckBudget(); err != nil {
		return nil, err
	}

	stream, err := c.client.CreateChatCompletionStream(ctx, req)
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
	stream, err := c.client.CreateChatCompletionStream(ctx, openai.ChatCompletionRequest{
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
		resp, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return full, &gperr.APIError{Op: "stream", Message: "recv error", Err: err}
		}
		if resp.Usage != nil {
			streamUsage = resp.Usage
		}
		if len(resp.Choices) > 0 {
			chunk := resp.Choices[0].Delta.Content
			full += chunk
			if onChunk != nil {
				onChunk(chunk)
			}
		}
	}

	if streamUsage != nil {
		usage.Global().RecordUsage(model, streamUsage.PromptTokens, streamUsage.CompletionTokens)
		usage.Global().WarnIfNeeded()
	}

	return full, nil
}

// CompleteVision sends a vision request with an image (base64 data URI) and text question.
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

	resp, err := c.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
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

// ImageResult holds a generated image's data.
type ImageResult struct {
	B64JSON       string
	RevisedPrompt string
}

// CreateImage generates images using the OpenAI images API.
func (c *Client) CreateImage(ctx context.Context, prompt, model, size string, n int) ([]ImageResult, error) {
	if err := usage.Global().CheckBudget(); err != nil {
		return nil, err
	}

	req := openai.ImageRequest{
		Prompt: prompt,
		Model:  model,
		N:      n,
		Size:   size,
	}

	// gpt-image-* models use output_format; DALL-E models use response_format
	if strings.HasPrefix(model, "gpt-image") {
		req.OutputFormat = openai.CreateImageOutputFormatPNG
	} else {
		req.ResponseFormat = openai.CreateImageResponseFormatB64JSON
	}

	resp, err := c.client.CreateImage(ctx, req)
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
	list, err := c.client.ListModels(ctx)
	if err != nil {
		return nil, &gperr.APIError{Op: "list-models", Message: "API error", Err: err}
	}
	ids := make([]string, len(list.Models))
	for i, m := range list.Models {
		ids[i] = m.ID
	}
	sort.Strings(ids)
	return ids, nil
}

func (c *Client) CreateTranscription(ctx context.Context, request openai.AudioRequest) (openai.AudioResponse, error) {
	if err := usage.Global().CheckBudget(); err != nil {
		return openai.AudioResponse{}, err
	}

	resp, err := c.client.CreateTranscription(ctx, request)
	if err != nil {
		return openai.AudioResponse{}, &gperr.APIError{Op: "transcription", Message: "API error", Err: err}
	}
	return resp, nil
}

func (c *Client) CreateTranslation(ctx context.Context, request openai.AudioRequest) (openai.AudioResponse, error) {
	if err := usage.Global().CheckBudget(); err != nil {
		return openai.AudioResponse{}, err
	}

	resp, err := c.client.CreateTranslation(ctx, request)
	if err != nil {
		return openai.AudioResponse{}, &gperr.APIError{Op: "translation", Message: "API error", Err: err}
	}
	return resp, nil
}

func (c *Client) CreateSpeech(ctx context.Context, request openai.CreateSpeechRequest) ([]byte, error) {
	if err := usage.Global().CheckBudget(); err != nil {
		return nil, err
	}

	resp, err := c.client.CreateSpeech(ctx, request)
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
