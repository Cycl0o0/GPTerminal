package chat

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type savedMessage struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	Timestamp string `json:"timestamp"`
}

type chatHistory struct {
	Messages []savedMessage `json:"messages"`
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
			Role:    m.role,
			Content: m.content,
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
