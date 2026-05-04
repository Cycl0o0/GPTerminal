package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"github.com/google/generative-ai-go/genai"
	openai "github.com/sashabaranov/go-openai"
	"google.golang.org/api/iterator"
	goption "google.golang.org/api/option"
)

type GeminiProvider struct {
	apiKey string
}

func NewGeminiProvider(apiKey string) *GeminiProvider {
	return &GeminiProvider{apiKey: apiKey}
}

func (p *GeminiProvider) Name() string { return "gemini" }

func (p *GeminiProvider) newClient(ctx context.Context) (*genai.Client, error) {
	return genai.NewClient(ctx, goption.WithAPIKey(p.apiKey))
}

func (p *GeminiProvider) CreateChatCompletion(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	client, err := p.newClient(ctx)
	if err != nil {
		return openai.ChatCompletionResponse{}, fmt.Errorf("gemini: %w", err)
	}
	defer client.Close()

	model := client.GenerativeModel(req.Model)
	p.configureModel(model, req)

	history, userParts := p.convertMessages(req.Messages)
	cs := model.StartChat()
	cs.History = history

	resp, err := cs.SendMessage(ctx, userParts...)
	if err != nil {
		return openai.ChatCompletionResponse{}, fmt.Errorf("gemini: %w", err)
	}

	return p.convertResponse(resp, req.Model), nil
}

func (p *GeminiProvider) CreateChatCompletionStream(ctx context.Context, req openai.ChatCompletionRequest) (ChatStream, error) {
	client, err := p.newClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("gemini: %w", err)
	}

	model := client.GenerativeModel(req.Model)
	p.configureModel(model, req)

	history, userParts := p.convertMessages(req.Messages)
	cs := model.StartChat()
	cs.History = history

	iter := cs.SendMessageStream(ctx, userParts...)
	return &geminiStream{iter: iter, client: client}, nil
}

func (p *GeminiProvider) ListModels(ctx context.Context) ([]string, error) {
	client, err := p.newClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("gemini: %w", err)
	}
	defer client.Close()

	iter := client.ListModels(ctx)
	var models []string
	for {
		mi, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			break
		}
		models = append(models, mi.Name)
	}
	sort.Strings(models)
	return models, nil
}

func (p *GeminiProvider) configureModel(model *genai.GenerativeModel, req openai.ChatCompletionRequest) {
	if req.Temperature > 0 {
		temp := float32(req.Temperature)
		model.SetTemperature(temp)
	}
	if req.MaxTokens > 0 {
		model.SetMaxOutputTokens(int32(req.MaxTokens))
	}

	if len(req.Tools) > 0 {
		var decls []*genai.FunctionDeclaration
		for _, t := range req.Tools {
			if t.Function == nil {
				continue
			}
			fd := &genai.FunctionDeclaration{
				Name:        t.Function.Name,
				Description: t.Function.Description,
			}
			if paramsMap, ok := t.Function.Parameters.(map[string]any); ok {
				fd.Parameters = convertToGeminiSchema(paramsMap)
			}
			decls = append(decls, fd)
		}
		model.Tools = []*genai.Tool{{FunctionDeclarations: decls}}
	}
}

func convertToGeminiSchema(params map[string]any) *genai.Schema {
	schema := &genai.Schema{Type: genai.TypeObject}
	if props, ok := params["properties"].(map[string]any); ok {
		schema.Properties = make(map[string]*genai.Schema)
		for name, prop := range props {
			if propMap, ok := prop.(map[string]any); ok {
				schema.Properties[name] = convertPropertyToSchema(propMap)
			}
		}
	}
	if required, ok := params["required"].([]string); ok {
		schema.Required = required
	} else if reqAny, ok := params["required"].([]any); ok {
		for _, r := range reqAny {
			if s, ok := r.(string); ok {
				schema.Required = append(schema.Required, s)
			}
		}
	}
	return schema
}

func convertPropertyToSchema(prop map[string]any) *genai.Schema {
	s := &genai.Schema{}
	if t, ok := prop["type"].(string); ok {
		switch t {
		case "string":
			s.Type = genai.TypeString
		case "number":
			s.Type = genai.TypeNumber
		case "integer":
			s.Type = genai.TypeInteger
		case "boolean":
			s.Type = genai.TypeBoolean
		case "array":
			s.Type = genai.TypeArray
			if items, ok := prop["items"].(map[string]any); ok {
				s.Items = convertPropertyToSchema(items)
			}
		case "object":
			s.Type = genai.TypeObject
			if props, ok := prop["properties"].(map[string]any); ok {
				s.Properties = make(map[string]*genai.Schema)
				for name, p := range props {
					if pm, ok := p.(map[string]any); ok {
						s.Properties[name] = convertPropertyToSchema(pm)
					}
				}
			}
		}
	}
	if desc, ok := prop["description"].(string); ok {
		s.Description = desc
	}
	return s
}

func (p *GeminiProvider) convertMessages(msgs []openai.ChatCompletionMessage) ([]*genai.Content, []genai.Part) {
	var history []*genai.Content
	var systemText string

	for _, msg := range msgs {
		switch msg.Role {
		case "system":
			systemText += msg.Content + "\n"
		case "user":
			history = append(history, &genai.Content{
				Role:  "user",
				Parts: []genai.Part{genai.Text(msg.Content)},
			})
		case "assistant":
			var parts []genai.Part
			if msg.Content != "" {
				parts = append(parts, genai.Text(msg.Content))
			}
			for _, tc := range msg.ToolCalls {
				var args map[string]any
				_ = json.Unmarshal([]byte(tc.Function.Arguments), &args)
				parts = append(parts, genai.FunctionCall{
					Name: tc.Function.Name,
					Args: args,
				})
			}
			if len(parts) > 0 {
				history = append(history, &genai.Content{
					Role:  "model",
					Parts: parts,
				})
			}
		case "tool":
			history = append(history, &genai.Content{
				Role: "user",
				Parts: []genai.Part{genai.FunctionResponse{
					Name:     msg.Name,
					Response: map[string]any{"result": msg.Content},
				}},
			})
		}
	}

	if systemText != "" && len(history) > 0 {
		first := history[0]
		if first.Role == "user" {
			first.Parts = append([]genai.Part{genai.Text("[System]\n" + systemText)}, first.Parts...)
		}
	}

	if len(history) == 0 {
		return nil, []genai.Part{genai.Text("Hello")}
	}

	last := history[len(history)-1]
	if last.Role != "user" {
		return history, []genai.Part{genai.Text("Continue")}
	}
	userParts := last.Parts
	history = history[:len(history)-1]
	return history, userParts
}

func (p *GeminiProvider) convertResponse(resp *genai.GenerateContentResponse, model string) openai.ChatCompletionResponse {
	var content string
	var toolCalls []openai.ToolCall
	callIdx := 0

	if resp != nil {
		for _, cand := range resp.Candidates {
			if cand.Content == nil {
				continue
			}
			for _, part := range cand.Content.Parts {
				switch v := part.(type) {
				case genai.Text:
					content += string(v)
				case genai.FunctionCall:
					args, _ := json.Marshal(v.Args)
					toolCalls = append(toolCalls, openai.ToolCall{
						ID:   fmt.Sprintf("call_%d", callIdx),
						Type: openai.ToolTypeFunction,
						Function: openai.FunctionCall{
							Name:      v.Name,
							Arguments: string(args),
						},
					})
					callIdx++
				}
			}
		}
	}

	usage := openai.Usage{}
	if resp != nil && resp.UsageMetadata != nil {
		usage.PromptTokens = int(resp.UsageMetadata.PromptTokenCount)
		usage.CompletionTokens = int(resp.UsageMetadata.CandidatesTokenCount)
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
		Usage: usage,
	}
}

type geminiStream struct {
	iter    *genai.GenerateContentResponseIterator
	client  *genai.Client
	callIdx int
}

func (s *geminiStream) Recv() (ChatStreamEvent, error) {
	resp, err := s.iter.Next()
	if err == iterator.Done {
		s.client.Close()
		return ChatStreamEvent{}, io.EOF
	}
	if err != nil {
		s.client.Close()
		return ChatStreamEvent{}, err
	}

	var evt ChatStreamEvent
	for _, cand := range resp.Candidates {
		if cand.Content == nil {
			continue
		}
		for _, part := range cand.Content.Parts {
			switch v := part.(type) {
			case genai.Text:
				evt.Content += string(v)
			case genai.FunctionCall:
				args, _ := json.Marshal(v.Args)
				evt.ToolCalls = append(evt.ToolCalls, openai.ToolCall{
					ID:   fmt.Sprintf("call_%d", s.callIdx),
					Type: openai.ToolTypeFunction,
					Function: openai.FunctionCall{
						Name:      v.Name,
						Arguments: string(args),
					},
				})
				s.callIdx++
			}
		}
	}

	if resp.UsageMetadata != nil {
		evt.Usage = &openai.Usage{
			PromptTokens:     int(resp.UsageMetadata.PromptTokenCount),
			CompletionTokens: int(resp.UsageMetadata.CandidatesTokenCount),
		}
	}

	return evt, nil
}

func (s *geminiStream) Close() {
	s.client.Close()
}
