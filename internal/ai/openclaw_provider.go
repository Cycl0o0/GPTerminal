package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/a3tai/openclaw-go/gateway"
	"github.com/a3tai/openclaw-go/protocol"
	openai "github.com/sashabaranov/go-openai"
)

type OpenClawProvider struct {
	baseURL string
	token   string
	agent   string

	mu     sync.Mutex
	client *gateway.Client
}

func NewOpenClawProvider(baseURL, token, agent string) *OpenClawProvider {
	return &OpenClawProvider{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		agent:   agent,
	}
}

func (p *OpenClawProvider) Name() string { return "openclaw" }

// wsURL converts http(s) base URL to ws(s) URL.
func (p *OpenClawProvider) wsURL() string {
	u := p.baseURL
	u = strings.Replace(u, "https://", "wss://", 1)
	u = strings.Replace(u, "http://", "ws://", 1)
	if !strings.HasSuffix(u, "/ws") {
		u += "/ws"
	}
	return u
}

func (p *OpenClawProvider) ensureClient(ctx context.Context, eventCh chan<- chatEventMsg) (*gateway.Client, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Close stale client.
	if p.client != nil {
		p.client.Close()
		p.client = nil
	}

	client := gateway.NewClient(
		gateway.WithPassword(p.token),
		gateway.WithRole(protocol.RoleOperator),
		gateway.WithScopes(protocol.ScopeOperatorRead, protocol.ScopeOperatorWrite),
		gateway.WithOnEvent(func(ev protocol.Event) {
			if ev.EventName == protocol.EventChat {
				eventCh <- chatEventMsg{payload: ev.Payload}
			}
		}),
	)

	connectCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	if err := client.Connect(connectCtx, p.wsURL()); err != nil {
		client.Close()
		return nil, fmt.Errorf("openclaw: connect: %w", err)
	}

	p.client = client
	return client, nil
}

type chatEventMsg struct {
	payload json.RawMessage
}

func (p *OpenClawProvider) CreateChatCompletion(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	stream, err := p.CreateChatCompletionStream(ctx, req)
	if err != nil {
		return openai.ChatCompletionResponse{}, err
	}
	defer stream.Close()

	var content strings.Builder
	for {
		evt, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return openai.ChatCompletionResponse{}, err
		}
		content.WriteString(evt.Content)
	}

	return openai.ChatCompletionResponse{
		Choices: []openai.ChatCompletionChoice{
			{
				Message: openai.ChatCompletionMessage{
					Role:    openai.ChatMessageRoleAssistant,
					Content: content.String(),
				},
			},
		},
	}, nil
}

func (p *OpenClawProvider) CreateChatCompletionStream(ctx context.Context, req openai.ChatCompletionRequest) (ChatStream, error) {
	// Only send the latest user message.
	var userContent string
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == openai.ChatMessageRoleUser {
			userContent = req.Messages[i].Content
			break
		}
	}
	if userContent == "" {
		return nil, fmt.Errorf("openclaw: no user message found")
	}

	eventCh := make(chan chatEventMsg, 64)

	client, err := p.ensureClient(ctx, eventCh)
	if err != nil {
		return nil, err
	}

	sessionKey := "gpterminal"
	if p.agent != "" {
		sessionKey = "agent:" + p.agent + ":gpterminal"
	}

	_, err = client.ChatSend(ctx, protocol.ChatSendParams{
		SessionKey:     sessionKey,
		Message:        userContent,
		IdempotencyKey: fmt.Sprintf("gpt-%d", time.Now().UnixNano()),
	})
	if err != nil {
		return nil, fmt.Errorf("openclaw: chat.send: %w", err)
	}

	return &openClawStream{
		eventCh: eventCh,
		client:  client,
		done:    false,
	}, nil
}

func (p *OpenClawProvider) ListModels(ctx context.Context) ([]string, error) {
	name := p.agent
	if name == "" {
		name = "default"
	}
	return []string{name}, nil
}

// openClawStream reads "chat" events from the WebSocket event handler.
type openClawStream struct {
	eventCh <-chan chatEventMsg
	client  *gateway.Client
	done    bool
}

// extractText tries to read the message field from a ChatEvent payload.
// The "message" field is json.RawMessage — it may be a plain JSON string,
// a structured content-block array, or absent entirely.
func extractText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	// Fast path: JSON string (most common for delta/final).
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	// Fallback: array of content blocks [{type:"text",text:"..."},...].
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &blocks) == nil {
		var buf strings.Builder
		for _, b := range blocks {
			if b.Type == "text" {
				buf.WriteString(b.Text)
			}
		}
		return buf.String()
	}
	// Last resort: stringify whatever it is.
	return string(raw)
}

func (s *openClawStream) Recv() (ChatStreamEvent, error) {
	if s.done {
		return ChatStreamEvent{}, io.EOF
	}

	for {
		select {
		case msg, ok := <-s.eventCh:
			if !ok {
				s.done = true
				return ChatStreamEvent{}, io.EOF
			}

			var ev protocol.ChatEvent
			if json.Unmarshal(msg.payload, &ev) != nil {
				// Also try map fallback for tool_use/tool_result events
				// that may have extra fields not in ChatEvent.
				var data map[string]any
				if json.Unmarshal(msg.payload, &data) != nil {
					continue
				}
				state, _ := data["state"].(string)
				switch state {
				case "tool_use":
					name, _ := data["toolName"].(string)
					args, _ := data["toolArgs"].(string)
					if argsMap, ok := data["toolArgs"].(map[string]any); ok {
						b, _ := json.Marshal(argsMap)
						args = string(b)
					}
					if name != "" {
						return ChatStreamEvent{
							ServerToolCall: &ServerToolEvent{Name: name, Arguments: args},
						}, nil
					}
				case "tool_result":
					name, _ := data["toolName"].(string)
					result, _ := data["toolResult"].(string)
					if name != "" {
						return ChatStreamEvent{
							ServerToolResult: &ServerToolEvent{Name: name, Result: result},
						}, nil
					}
				}
				continue
			}

			switch ev.State {
			case "delta":
				text := extractText(ev.Message)
				if text != "" {
					return ChatStreamEvent{Content: text}, nil
				}

			case "final":
				s.done = true
				text := extractText(ev.Message)
				if text != "" {
					return ChatStreamEvent{Content: text}, nil
				}
				return ChatStreamEvent{}, io.EOF

			case "error":
				s.done = true
				return ChatStreamEvent{}, fmt.Errorf("openclaw: agent error: %s", ev.ErrorMessage)

			case "aborted":
				s.done = true
				return ChatStreamEvent{}, io.EOF

			case "tool_use", "tool_result":
				// Typed ChatEvent doesn't have tool fields — fall through to map parse.
				var data map[string]any
				if json.Unmarshal(msg.payload, &data) != nil {
					continue
				}
				if ev.State == "tool_use" {
					name, _ := data["toolName"].(string)
					args, _ := data["toolArgs"].(string)
					if argsMap, ok := data["toolArgs"].(map[string]any); ok {
						b, _ := json.Marshal(argsMap)
						args = string(b)
					}
					if name != "" {
						return ChatStreamEvent{
							ServerToolCall: &ServerToolEvent{Name: name, Arguments: args},
						}, nil
					}
				} else {
					name, _ := data["toolName"].(string)
					result, _ := data["toolResult"].(string)
					if name != "" {
						return ChatStreamEvent{
							ServerToolResult: &ServerToolEvent{Name: name, Result: result},
						}, nil
					}
				}
			}

		case <-s.client.Done():
			s.done = true
			return ChatStreamEvent{}, io.EOF
		}
	}
}

func (s *openClawStream) Close() {
	s.done = true
}
