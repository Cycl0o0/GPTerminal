package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/packages/param"
	"github.com/anthropics/anthropic-sdk-go/packages/ssestream"
	openai "github.com/sashabaranov/go-openai"
)

type AnthropicProvider struct {
	client anthropic.Client
}

func NewAnthropicProvider(apiKey string) *AnthropicProvider {
	return &AnthropicProvider{
		client: anthropic.NewClient(option.WithAPIKey(apiKey)),
	}
}

func (p *AnthropicProvider) Name() string { return "anthropic" }

func (p *AnthropicProvider) CreateChatCompletion(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	params := p.convertRequest(req)
	msg, err := p.client.Messages.New(ctx, params)
	if err != nil {
		return openai.ChatCompletionResponse{}, fmt.Errorf("anthropic: %w", err)
	}
	return p.convertResponse(msg, req.Model), nil
}

func (p *AnthropicProvider) CreateChatCompletionStream(ctx context.Context, req openai.ChatCompletionRequest) (ChatStream, error) {
	params := p.convertRequest(req)
	stream := p.client.Messages.NewStreaming(ctx, params)
	return &anthropicStream{stream: stream}, nil
}

func (p *AnthropicProvider) ListModels(ctx context.Context) ([]string, error) {
	models := []string{
		"claude-sonnet-4-20250514",
		"claude-haiku-4-20250414",
		"claude-opus-4-20250514",
		"claude-3-5-sonnet-20241022",
		"claude-3-5-haiku-20241022",
	}
	sort.Strings(models)
	return models, nil
}

func (p *AnthropicProvider) convertRequest(req openai.ChatCompletionRequest) anthropic.MessageNewParams {
	var system []anthropic.TextBlockParam
	var messages []anthropic.MessageParam

	for _, msg := range req.Messages {
		switch msg.Role {
		case "system":
			system = append(system, anthropic.TextBlockParam{Text: msg.Content})
		case "user":
			messages = append(messages, anthropic.NewUserMessage(anthropic.NewTextBlock(msg.Content)))
		case "assistant":
			var blocks []anthropic.ContentBlockParamUnion
			if msg.Content != "" {
				blocks = append(blocks, anthropic.NewTextBlock(msg.Content))
			}
			for _, tc := range msg.ToolCalls {
				var input any
				_ = json.Unmarshal([]byte(tc.Function.Arguments), &input)
				blocks = append(blocks, anthropic.NewToolUseBlock(tc.ID, input, tc.Function.Name))
			}
			if len(blocks) > 0 {
				messages = append(messages, anthropic.NewAssistantMessage(blocks...))
			}
		case "tool":
			messages = append(messages, anthropic.NewUserMessage(
				anthropic.NewToolResultBlock(msg.ToolCallID, msg.Content, false),
			))
		}
	}

	maxTokens := int64(req.MaxTokens)
	if maxTokens <= 0 {
		maxTokens = 4096
	}

	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(req.Model),
		MaxTokens: maxTokens,
		Messages:  messages,
		System:    system,
	}

	if req.Temperature > 0 {
		params.Temperature = param.NewOpt(float64(req.Temperature))
	}

	if len(req.Tools) > 0 {
		for _, t := range req.Tools {
			if t.Function == nil {
				continue
			}
			paramsMap, _ := t.Function.Parameters.(map[string]any)
			var props any
			var required []string
			if paramsMap != nil {
				props = paramsMap["properties"]
				if reqSlice, ok := paramsMap["required"].([]string); ok {
					required = reqSlice
				} else if reqAny, ok := paramsMap["required"].([]any); ok {
					for _, r := range reqAny {
						if s, ok := r.(string); ok {
							required = append(required, s)
						}
					}
				}
			}

			tool := anthropic.ToolParam{
				Name: t.Function.Name,
				InputSchema: anthropic.ToolInputSchemaParam{
					Properties: props,
					Required:   required,
				},
			}
			if t.Function.Description != "" {
				tool.Description = param.NewOpt(t.Function.Description)
			}
			params.Tools = append(params.Tools, anthropic.ToolUnionParam{OfTool: &tool})
		}
	}

	return params
}

func (p *AnthropicProvider) convertResponse(msg *anthropic.Message, model string) openai.ChatCompletionResponse {
	var content string
	var toolCalls []openai.ToolCall

	for _, block := range msg.Content {
		switch block.Type {
		case "text":
			content += block.Text
		case "tool_use":
			tu := block.AsToolUse()
			args, _ := json.Marshal(tu.Input)
			toolCalls = append(toolCalls, openai.ToolCall{
				ID:   tu.ID,
				Type: openai.ToolTypeFunction,
				Function: openai.FunctionCall{
					Name:      tu.Name,
					Arguments: string(args),
				},
			})
		}
	}

	return openai.ChatCompletionResponse{
		Model: model,
		Choices: []openai.ChatCompletionChoice{{
			Message: openai.ChatCompletionMessage{
				Role:      openai.ChatMessageRoleAssistant,
				Content:   content,
				ToolCalls: toolCalls,
			},
		}},
		Usage: openai.Usage{
			PromptTokens:     int(msg.Usage.InputTokens),
			CompletionTokens: int(msg.Usage.OutputTokens),
		},
	}
}

type anthropicStream struct {
	stream      *ssestream.Stream[anthropic.MessageStreamEventUnion]
	toolCalls   map[int]*openai.ToolCall
	toolInputs  map[int]*strings.Builder
	inputTokens int
}

func (s *anthropicStream) Recv() (ChatStreamEvent, error) {
	if s.toolCalls == nil {
		s.toolCalls = make(map[int]*openai.ToolCall)
		s.toolInputs = make(map[int]*strings.Builder)
	}

	for s.stream.Next() {
		evt := s.stream.Current()
		switch evt.Type {
		case "message_start":
			msg := evt.AsMessageStart()
			s.inputTokens = int(msg.Message.Usage.InputTokens)
			continue

		case "content_block_start":
			cbs := evt.AsContentBlockStart()
			idx := int(cbs.Index)
			if cbs.ContentBlock.Type == "tool_use" {
				tu := cbs.ContentBlock.AsToolUse()
				s.toolCalls[idx] = &openai.ToolCall{
					ID:   tu.ID,
					Type: openai.ToolTypeFunction,
					Function: openai.FunctionCall{
						Name: tu.Name,
					},
				}
				s.toolInputs[idx] = &strings.Builder{}
			}
			continue

		case "content_block_delta":
			cbd := evt.AsContentBlockDelta()
			idx := int(cbd.Index)
			switch cbd.Delta.Type {
			case "text_delta":
				return ChatStreamEvent{Content: cbd.Delta.Text}, nil
			case "thinking_delta":
				return ChatStreamEvent{ReasoningContent: cbd.Delta.Thinking}, nil
			case "input_json_delta":
				if sb, ok := s.toolInputs[idx]; ok {
					sb.WriteString(cbd.Delta.PartialJSON)
				}
			}
			continue

		case "content_block_stop":
			idx := int(evt.Index)
			if tc, ok := s.toolCalls[idx]; ok {
				if sb, ok := s.toolInputs[idx]; ok {
					tc.Function.Arguments = sb.String()
				}
				calls := []openai.ToolCall{*tc}
				delete(s.toolCalls, idx)
				delete(s.toolInputs, idx)
				return ChatStreamEvent{ToolCalls: calls}, nil
			}
			continue

		case "message_delta":
			md := evt.AsMessageDelta()
			return ChatStreamEvent{
				Usage: &openai.Usage{
					PromptTokens:     s.inputTokens,
					CompletionTokens: int(md.Usage.OutputTokens),
				},
			}, nil

		case "message_stop":
			return ChatStreamEvent{}, io.EOF
		}
	}

	if err := s.stream.Err(); err != nil {
		return ChatStreamEvent{}, err
	}
	return ChatStreamEvent{}, io.EOF
}

func (s *anthropicStream) Close() {
	s.stream.Close()
}
