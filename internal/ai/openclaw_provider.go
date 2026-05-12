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
		gateway.WithCaps(protocol.ClientCapToolEvents),
		gateway.WithOnEvent(func(ev protocol.Event) {
			switch ev.EventName {
			case protocol.EventChat:
				eventCh <- chatEventMsg{payload: ev.Payload, eventName: "chat"}
			case protocol.EventAgent:
				eventCh <- chatEventMsg{payload: ev.Payload, eventName: "agent"}
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
	payload   json.RawMessage
	eventName string
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

	sessionKey := fmt.Sprintf("gpterminal-%d", time.Now().UnixMilli())
	if p.agent != "" {
		sessionKey = fmt.Sprintf("agent:%s:gpterminal-%d", p.agent, time.Now().UnixMilli())
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

// openClawStream reads "chat" and "agent" events from the WebSocket.
type openClawStream struct {
	eventCh   <-chan chatEventMsg
	client    *gateway.Client
	done      bool
	prevText  string // accumulated text — used to compute incremental diff
	seenTools int    // number of tool_use blocks already emitted
}

// contentBlock represents one block inside the message content array.
type contentBlock struct {
	Type  string          `json:"type"`
	Text  string          `json:"text,omitempty"`
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}

// chatMsg is the structure of the "message" field in chat events.
type chatMsg struct {
	Content []contentBlock `json:"content"`
}

// extractText reads only the text-type content from the message field.
func extractText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var msg chatMsg
	if json.Unmarshal(raw, &msg) == nil && len(msg.Content) > 0 {
		var buf strings.Builder
		for _, c := range msg.Content {
			if c.Type == "text" {
				buf.WriteString(c.Text)
			}
		}
		return buf.String()
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	return ""
}

// extractToolBlocks returns tool_use blocks from the message content.
func extractToolBlocks(raw json.RawMessage) []contentBlock {
	if len(raw) == 0 {
		return nil
	}
	var msg chatMsg
	if json.Unmarshal(raw, &msg) != nil {
		return nil
	}
	var tools []contentBlock
	for _, c := range msg.Content {
		if c.Type == "tool_use" {
			tools = append(tools, c)
		}
	}
	return tools
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

			// ── Agent events: tool calls, tool results, lifecycle ──
			if msg.eventName == "agent" {
				var ae protocol.AgentEvent
				if json.Unmarshal(msg.payload, &ae) != nil {
					continue
				}
				switch ae.Stream {
				case "tool_use":
					name, _ := ae.Data["name"].(string)
					if name == "" {
						name, _ = ae.Data["toolName"].(string)
					}
					args := ""
					if input, ok := ae.Data["input"]; ok {
						b, _ := json.Marshal(input)
						args = string(b)
					}
					if name != "" {
						return ChatStreamEvent{
							ServerToolCall: &ServerToolEvent{Name: name, Arguments: args},
						}, nil
					}
				case "tool_result":
					name, _ := ae.Data["name"].(string)
					if name == "" {
						name, _ = ae.Data["toolName"].(string)
					}
					result := ""
					if r, ok := ae.Data["result"]; ok {
						b, _ := json.Marshal(r)
						result = string(b)
					} else if r, ok := ae.Data["text"].(string); ok {
						result = r
					}
					if name != "" {
						return ChatStreamEvent{
							ServerToolResult: &ServerToolEvent{Name: name, Result: result},
						}, nil
					}
				}
				continue
			}

			// ── Chat events: delta, final, error, aborted ──
			var ev protocol.ChatEvent
			if json.Unmarshal(msg.payload, &ev) != nil {
				continue
			}

			switch ev.State {
			case "delta":
				// Check for tool_use content blocks in the accumulated message.
				tools := extractToolBlocks(ev.Message)
				if len(tools) > s.seenTools {
					tb := tools[s.seenTools]
					s.seenTools = len(tools)
					args := string(tb.Input)
					return ChatStreamEvent{
						ServerToolCall: &ServerToolEvent{Name: tb.Name, Arguments: args},
					}, nil
				}
				// Emit incremental text.
				full := extractText(ev.Message)
				if len(full) > len(s.prevText) {
					delta := full[len(s.prevText):]
					s.prevText = full
					return ChatStreamEvent{Content: delta}, nil
				}

			case "final":
				s.done = true
				full := extractText(ev.Message)
				if len(full) > len(s.prevText) {
					delta := full[len(s.prevText):]
					return ChatStreamEvent{Content: delta}, nil
				}
				return ChatStreamEvent{}, io.EOF

			case "error":
				s.done = true
				return ChatStreamEvent{}, fmt.Errorf("openclaw: agent error: %s", ev.ErrorMessage)

			case "aborted":
				s.done = true
				return ChatStreamEvent{}, io.EOF
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
