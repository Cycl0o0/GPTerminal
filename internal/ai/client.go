package ai

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/cycl0o0/GPTerminal/internal/config"
	openai "github.com/sashabaranov/go-openai"
)

type Client struct {
	client *openai.Client
}

func NewClient() (*Client, error) {
	key := config.APIKey()
	if key == "" {
		return nil, fmt.Errorf("OpenAI API key not set. Run: gpterminal config set-key <key>\nOr set OPENAI_API_KEY environment variable")
	}
	return &Client{client: openai.NewClient(key)}, nil
}

func (c *Client) Complete(ctx context.Context, messages []openai.ChatCompletionMessage) (string, error) {
	resp, err := c.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:       config.Model(),
		Messages:    messages,
		Temperature: config.Temperature(),
		MaxTokens:   config.MaxTokens(),
	})
	if err != nil {
		return "", fmt.Errorf("OpenAI API error: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response from OpenAI")
	}
	return resp.Choices[0].Message.Content, nil
}

func (c *Client) StreamComplete(ctx context.Context, messages []openai.ChatCompletionMessage, onChunk func(string)) (string, error) {
	stream, err := c.client.CreateChatCompletionStream(ctx, openai.ChatCompletionRequest{
		Model:       config.Model(),
		Messages:    messages,
		Temperature: config.Temperature(),
		MaxTokens:   config.MaxTokens(),
		Stream:      true,
	})
	if err != nil {
		return "", fmt.Errorf("OpenAI stream error: %w", err)
	}
	defer stream.Close()

	var full string
	for {
		resp, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return full, fmt.Errorf("stream recv error: %w", err)
		}
		chunk := resp.Choices[0].Delta.Content
		full += chunk
		if onChunk != nil {
			onChunk(chunk)
		}
	}
	return full, nil
}
