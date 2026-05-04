package memory

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Entry struct {
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	CreatedAt time.Time `json:"created_at"`
}

type Store struct {
	Entries []Entry `json:"entries"`
}

func storeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".config", "gpterminal")
}

func storePath() string {
	return filepath.Join(storeDir(), "memory.json")
}

func Load() (*Store, error) {
	data, err := os.ReadFile(storePath())
	if err != nil {
		if os.IsNotExist(err) {
			return &Store{}, nil
		}
		return nil, err
	}
	var s Store
	if err := json.Unmarshal(data, &s); err != nil {
		return &Store{}, nil
	}
	return &s, nil
}

func (s *Store) Save() error {
	dir := storeDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(storePath(), data, 0600)
}

func (s *Store) Set(key, value string) {
	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)
	if key == "" {
		return
	}
	for i, e := range s.Entries {
		if strings.EqualFold(e.Key, key) {
			s.Entries[i].Value = value
			s.Entries[i].CreatedAt = time.Now().UTC()
			return
		}
	}
	s.Entries = append(s.Entries, Entry{
		Key:       key,
		Value:     value,
		CreatedAt: time.Now().UTC(),
	})
}

func (s *Store) Delete(key string) bool {
	key = strings.TrimSpace(key)
	for i, e := range s.Entries {
		if strings.EqualFold(e.Key, key) {
			s.Entries = append(s.Entries[:i], s.Entries[i+1:]...)
			return true
		}
	}
	return false
}

func (s *Store) Search(query string) []Entry {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return s.Entries
	}
	var results []Entry
	for _, e := range s.Entries {
		if strings.Contains(strings.ToLower(e.Key), query) ||
			strings.Contains(strings.ToLower(e.Value), query) {
			results = append(results, e)
		}
	}
	return results
}

func (s *Store) ContextBlock() string {
	if len(s.Entries) == 0 {
		return ""
	}
	sorted := make([]Entry, len(s.Entries))
	copy(sorted, s.Entries)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].CreatedAt.Before(sorted[j].CreatedAt)
	})

	var b strings.Builder
	b.WriteString("Remembered context from previous conversations:\n")
	for _, e := range sorted {
		b.WriteString(fmt.Sprintf("- %s: %s\n", e.Key, e.Value))
	}
	return b.String()
}

func (s *Store) List() string {
	if len(s.Entries) == 0 {
		return "No memories stored."
	}
	var b strings.Builder
	for _, e := range s.Entries {
		b.WriteString(fmt.Sprintf("- %s: %s\n", e.Key, e.Value))
	}
	return b.String()
}
