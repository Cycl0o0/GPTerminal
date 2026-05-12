package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

type OpenClawProvider struct {
	baseURL    string
	token      string
	username   string
	password   string
	agent      string
	httpClient *http.Client
	sessionID  string
}

func NewOpenClawProvider(baseURL, token, agent string) *OpenClawProvider {
	return &OpenClawProvider{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		agent:   agent,
		httpClient: &http.Client{
			Timeout: 5 * time.Minute,
		},
		sessionID: "default",
	}
}

func NewOpenClawProviderWithPassword(baseURL, username, password, agent string) *OpenClawProvider {
	return &OpenClawProvider{
		baseURL:  strings.TrimRight(baseURL, "/"),
		username: username,
		password: password,
		agent:    agent,
		httpClient: &http.Client{
			Timeout: 5 * time.Minute,
		},
		sessionID: "default",
	}
}

// login exchanges username/password for a Bearer token via the auth endpoint.
func (p *OpenClawProvider) login() error {
	body, _ := json.Marshal(map[string]string{
		"username": p.username,
		"password": p.password,
	})
	resp, err := p.httpClient.Post(
		p.baseURL+"/api/auth/login",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("openclaw login: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("openclaw login: HTTP %d: %s", resp.StatusCode, string(errBody))
	}

	var result struct {
		Token       string `json:"token"`
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("openclaw login: decode response: %w", err)
	}
	p.token = result.Token
	if p.token == "" {
		p.token = result.AccessToken
	}
	if p.token == "" {
		return fmt.Errorf("openclaw login: no token in response")
	}
	return nil
}

// ensureToken logs in with password if no token is present.
func (p *OpenClawProvider) ensureToken() error {
	if p.token != "" {
		return nil
	}
	if p.username == "" || p.password == "" {
		return fmt.Errorf("openclaw: no token or username/password configured")
	}
	return p.login()
}

func (p *OpenClawProvider) Name() string { return "openclaw" }

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
	if err := p.ensureToken(); err != nil {
		return nil, err
	}

	// Only send the latest user message — OpenClaw manages its own history.
	var userContent string
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == openai.ChatMessageRoleUser {
			userContent = req.Messages[i].Content
			break
		}
	}
	if userContent == "" {
		return nil, fmt.Errorf("openclaw: no user message found in request")
	}

	body := map[string]any{
		"message": userContent,
	}
	if p.agent != "" {
		body["agent"] = p.agent
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("openclaw: marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/sessions/%s/messages", p.baseURL, p.sessionID)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("openclaw: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	if p.token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.token)
	}

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openclaw: request failed: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("openclaw: HTTP %d: %s", resp.StatusCode, string(errBody))
	}

	return &openClawStream{
		reader: bufio.NewReader(resp.Body),
		body:   resp.Body,
	}, nil
}

func (p *OpenClawProvider) ListModels(ctx context.Context) ([]string, error) {
	name := p.agent
	if name == "" {
		name = "default"
	}
	return []string{name}, nil
}

// openClawStream reads SSE events from the OpenClaw Gateway.
type openClawStream struct {
	reader *bufio.Reader
	body   io.ReadCloser
	done   bool
}

func (s *openClawStream) Recv() (ChatStreamEvent, error) {
	if s.done {
		return ChatStreamEvent{}, io.EOF
	}

	for {
		eventType, data, err := s.readSSEEvent()
		if err != nil {
			s.done = true
			if err == io.EOF {
				return ChatStreamEvent{}, io.EOF
			}
			return ChatStreamEvent{}, fmt.Errorf("openclaw: SSE read: %w", err)
		}

		evt, ok := s.mapEvent(eventType, data)
		if ok {
			return evt, nil
		}
		// Unrecognized event — skip and read next.
	}
}

func (s *openClawStream) Close() {
	s.done = true
	if s.body != nil {
		s.body.Close()
	}
}

// readSSEEvent reads one SSE event (event: + data: lines) from the stream.
func (s *openClawStream) readSSEEvent() (eventType string, data string, err error) {
	var dataLines []string

	for {
		line, err := s.reader.ReadString('\n')
		line = strings.TrimRight(line, "\r\n")

		if line == "" {
			// Empty line = end of event block.
			if len(dataLines) > 0 || eventType != "" {
				return eventType, strings.Join(dataLines, "\n"), nil
			}
			if err != nil {
				return "", "", err
			}
			continue
		}

		if strings.HasPrefix(line, "event:") {
			eventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		} else if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
		// Ignore comments (lines starting with ':') and other fields.

		if err != nil {
			if len(dataLines) > 0 || eventType != "" {
				return eventType, strings.Join(dataLines, "\n"), nil
			}
			return "", "", err
		}
	}
}

// mapEvent converts an SSE event type+data into a ChatStreamEvent.
func (s *openClawStream) mapEvent(eventType, data string) (ChatStreamEvent, bool) {
	switch eventType {
	case "message_stop", "done":
		s.done = true
		return ChatStreamEvent{}, false

	case "content_block_delta":
		var delta struct {
			Delta struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"delta"`
		}
		if json.Unmarshal([]byte(data), &delta) == nil && delta.Delta.Type == "text_delta" {
			return ChatStreamEvent{Content: delta.Delta.Text}, true
		}
		return ChatStreamEvent{}, false

	case "content_block_start":
		var block struct {
			ContentBlock struct {
				Type  string          `json:"type"`
				Name  string          `json:"name"`
				Input json.RawMessage `json:"input"`
			} `json:"content_block"`
		}
		if json.Unmarshal([]byte(data), &block) == nil && block.ContentBlock.Type == "tool_use" {
			return ChatStreamEvent{
				ServerToolCall: &ServerToolEvent{
					Name:      block.ContentBlock.Name,
					Arguments: string(block.ContentBlock.Input),
				},
			}, true
		}
		return ChatStreamEvent{}, false

	case "tool_result":
		var result struct {
			Name   string `json:"name"`
			Result string `json:"result"`
		}
		if json.Unmarshal([]byte(data), &result) == nil {
			return ChatStreamEvent{
				ServerToolResult: &ServerToolEvent{
					Name:   result.Name,
					Result: result.Result,
				},
			}, true
		}
		return ChatStreamEvent{}, false

	default:
		// For unrecognized event types, try to extract text content.
		if data != "" && eventType == "" {
			// Plain data-only SSE (some servers just send data: lines).
			var obj map[string]any
			if json.Unmarshal([]byte(data), &obj) == nil {
				if text, ok := obj["text"].(string); ok && text != "" {
					return ChatStreamEvent{Content: text}, true
				}
				if content, ok := obj["content"].(string); ok && content != "" {
					return ChatStreamEvent{Content: content}, true
				}
			}
			// Treat raw non-JSON data as text content.
			if !strings.HasPrefix(data, "{") {
				return ChatStreamEvent{Content: data}, true
			}
		}
		return ChatStreamEvent{}, false
	}
}
