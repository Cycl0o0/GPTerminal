package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

type Kind string

const (
	KindChat  Kind = "chat"
	KindGptDo Kind = "gptdo"
)

type ChatMessage struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	Timestamp string `json:"timestamp"`
}

type ChatData struct {
	Transcript []ChatMessage                  `json:"transcript"`
	History    []openai.ChatCompletionMessage `json:"history"`
}

type GptDoData struct {
	Request     string                         `json:"request"`
	Messages    []openai.ChatCompletionMessage `json:"messages"`
	CWD         string                         `json:"cwd"`
	AutoApprove bool                           `json:"auto_approve"`
	Completed   bool                           `json:"completed"`
	Summary     string                         `json:"summary"`
}

type Record struct {
	Kind      Kind       `json:"kind"`
	Name      string     `json:"name"`
	UpdatedAt time.Time  `json:"updated_at"`
	Chat      *ChatData  `json:"chat,omitempty"`
	GptDo     *GptDoData `json:"gptdo,omitempty"`
}

type Entry struct {
	Name           string
	Kind           Kind
	UpdatedAt      time.Time
	ChatMessages   int
	LastPreview    string
	GptDoRequest   string
	GptDoCompleted bool
	GptDoSummary   string
}

var invalidNameChars = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

func NormalizeName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	name = invalidNameChars.ReplaceAllString(name, "_")
	name = strings.Trim(name, "._-")
	if name == "" {
		return ""
	}
	return name
}

func BaseDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".config", "gpterminal", "sessions")
}

func Path(name string) (string, error) {
	name = NormalizeName(name)
	if name == "" {
		return "", fmt.Errorf("session name cannot be empty")
	}
	return filepath.Join(BaseDir(), name+".json"), nil
}

func Save(record *Record) error {
	if record == nil {
		return fmt.Errorf("session record is nil")
	}
	record.Name = NormalizeName(record.Name)
	if record.Name == "" {
		return fmt.Errorf("session name cannot be empty")
	}
	record.UpdatedAt = time.Now().UTC()

	path, err := Path(record.Name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func Load(name string) (*Record, error) {
	path, err := Path(name)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var record Record
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, err
	}
	record.Name = NormalizeName(record.Name)
	if record.Name == "" {
		record.Name = NormalizeName(name)
	}
	return &record, nil
}

func Export(name string) ([]byte, error) {
	record, err := Load(name)
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(record, "", "  ")
}

func List() ([]Entry, error) {
	dir := BaseDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	out := make([]Entry, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		record, err := Load(strings.TrimSuffix(entry.Name(), ".json"))
		if err != nil {
			return nil, err
		}
		out = append(out, entryFromRecord(record))
	}

	sortEntries(out)
	return out, nil
}

func Rename(oldName, newName string) error {
	oldPath, err := Path(oldName)
	if err != nil {
		return err
	}
	newPath, err := Path(newName)
	if err != nil {
		return err
	}
	if oldPath == newPath {
		return nil
	}
	if _, err := os.Stat(newPath); err == nil {
		return fmt.Errorf("session %q already exists", NormalizeName(newName))
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(oldPath, newPath); err != nil {
		return err
	}

	record, err := Load(newName)
	if err != nil {
		return err
	}
	record.Name = NormalizeName(newName)
	return Save(record)
}

func Delete(name string) error {
	path, err := Path(name)
	if err != nil {
		return err
	}
	return os.Remove(path)
}

func entryFromRecord(record *Record) Entry {
	entry := Entry{
		Name:      record.Name,
		Kind:      record.Kind,
		UpdatedAt: record.UpdatedAt,
	}

	switch record.Kind {
	case KindChat:
		if record.Chat != nil {
			entry.ChatMessages = len(record.Chat.Transcript)
			if n := len(record.Chat.Transcript); n > 0 {
				entry.LastPreview = preview(record.Chat.Transcript[n-1].Content)
			}
		}
	case KindGptDo:
		if record.GptDo != nil {
			entry.GptDoRequest = preview(record.GptDo.Request)
			entry.GptDoCompleted = record.GptDo.Completed
			entry.GptDoSummary = preview(record.GptDo.Summary)
		}
	}

	return entry
}

func preview(s string) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	if len(s) > 90 {
		return s[:90] + "..."
	}
	return s
}

func sortEntries(entries []Entry) {
	for i := 0; i < len(entries); i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[j].UpdatedAt.After(entries[i].UpdatedAt) {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}
}
