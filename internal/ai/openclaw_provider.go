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
	agent      string
	httpClient *http.Client
}

func NewOpenClawProvider(baseURL, token, agent string) *OpenClawProvider {
	return &OpenClawProvider{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		agent:   agent,
		httpClient: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}
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

	// OpenResponses API: POST /v1/responses
	model := "openclaw"
	if p.agent != "" {
		model = "openclaw/" + p.agent
	}

	body := map[string]any{
		"model":  model,
		"input":  userContent,
		"stream": true,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("openclaw: marshal request: %w", err)
	}

	url := p.baseURL + "/v1/responses"
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

// openClawStream reads SSE events from the OpenClaw Gateway OpenResponses API.
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
	}
}

func (s *openClawStream) Close() {
	s.done = true
	if s.body != nil {
		s.body.Close()
	}
}

// readSSEEvent reads one SSE event block from the stream.
func (s *openClawStream) readSSEEvent() (eventType string, data string, err error) {
	var dataLines []string

	for {
		line, readErr := s.reader.ReadString('\n')
		line = strings.TrimRight(line, "\r\n")

		if line == "" {
			if len(dataLines) > 0 || eventType != "" {
				return eventType, strings.Join(dataLines, "\n"), nil
			}
			if readErr != nil {
				return "", "", readErr
			}
			continue
		}

		if strings.HasPrefix(line, "event:") {
			eventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		} else if strings.HasPrefix(line, "data:") {
			d := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if d == "[DONE]" {
				s.done = true
				return "", "", io.EOF
			}
			dataLines = append(dataLines, d)
		}

		if readErr != nil {
			if len(dataLines) > 0 || eventType != "" {
				return eventType, strings.Join(dataLines, "\n"), nil
			}
			return "", "", readErr
		}
	}
}

// mapEvent converts an OpenResponses SSE event into a ChatStreamEvent.
func (s *openClawStream) mapEvent(eventType, data string) (ChatStreamEvent, bool) {
	switch eventType {
	case "response.output_text.delta":
		var delta struct {
			Delta string `json:"delta"`
		}
		if json.Unmarshal([]byte(data), &delta) == nil && delta.Delta != "" {
			return ChatStreamEvent{Content: delta.Delta}, true
		}
		return ChatStreamEvent{}, false

	case "response.completed":
		s.done = true
		return ChatStreamEvent{}, false

	default:
		return ChatStreamEvent{}, false
	}
}
