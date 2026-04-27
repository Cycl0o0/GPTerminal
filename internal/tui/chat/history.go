package chat

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/cycl0o0/GPTerminal/internal/session"
	openai "github.com/sashabaranov/go-openai"
)

type savedMessage struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	Timestamp string `json:"timestamp"`
}

type chatHistory struct {
	Messages []savedMessage `json:"messages"`
}

func loadChatState(sessionName string) ([]chatMessage, []openai.ChatCompletionMessage) {
	if sessionName != "" {
		record, err := session.Load(sessionName)
		if err == nil && record.Kind == session.KindChat && record.Chat != nil {
			messages := make([]chatMessage, 0, len(record.Chat.Transcript))
			for _, s := range record.Chat.Transcript {
				messages = append(messages, chatMessage{
					role:      s.Role,
					content:   s.Content,
					timestamp: s.Timestamp,
				})
			}
			history := make([]openai.ChatCompletionMessage, len(record.Chat.History))
			copy(history, record.Chat.History)
			return messages, history
		}
	}

	saved := loadHistory()
	if len(saved) == 0 {
		return nil, nil
	}
	messages := make([]chatMessage, 0, len(saved))
	history := make([]openai.ChatCompletionMessage, 0, len(saved))
	for _, s := range saved {
		messages = append(messages, chatMessage{
			role:      s.Role,
			content:   s.Content,
			timestamp: s.Timestamp,
		})
		history = append(history, openai.ChatCompletionMessage{
			Role:    s.Role,
			Content: s.Content,
		})
	}
	return messages, history
}

func saveChatState(messages []chatMessage, history []openai.ChatCompletionMessage, sessionName string) error {
	if sessionName != "" {
		transcript := make([]session.ChatMessage, len(messages))
		for i, m := range messages {
			transcript[i] = session.ChatMessage{
				Role:      m.role,
				Content:   m.content,
				Timestamp: m.timestamp,
			}
		}
		return session.Save(&session.Record{
			Kind: session.KindChat,
			Name: sessionName,
			Chat: &session.ChatData{
				Transcript: transcript,
				History:    history,
			},
		})
	}
	return saveHistory(messages)
}

func historyPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".config", "gpterminal", "chat_history.json")
}

func loadHistory() []savedMessage {
	data, err := os.ReadFile(historyPath())
	if err != nil {
		return nil
	}
	var h chatHistory
	if err := json.Unmarshal(data, &h); err != nil {
		return nil
	}
	return h.Messages
}

func saveHistory(messages []chatMessage) error {
	dir := filepath.Dir(historyPath())
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	saved := make([]savedMessage, len(messages))
	for i, m := range messages {
		saved[i] = savedMessage{
			Role:      m.role,
			Content:   m.content,
			Timestamp: m.timestamp,
		}
	}

	data, err := json.MarshalIndent(chatHistory{Messages: saved}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(historyPath(), data, 0600)
}

func nowTimestamp() string {
	return time.Now().Format("15:04")
}
