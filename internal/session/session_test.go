package session

import (
	"os"
	"testing"
	"time"
)

func TestListRenameDelete(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if err := Save(&Record{
		Kind: KindChat,
		Name: "chat-one",
		Chat: &ChatData{
			Transcript: []ChatMessage{{Role: "assistant", Content: "hello"}},
		},
	}); err != nil {
		t.Fatalf("save chat session: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	if err := Save(&Record{
		Kind: KindGptDo,
		Name: "plan-one",
		GptDo: &GptDoData{
			Request:   "create a file",
			Completed: true,
			Summary:   "done",
		},
	}); err != nil {
		t.Fatalf("save gptdo session: %v", err)
	}

	entries, err := List()
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(entries))
	}
	if entries[0].Name != "plan-one" {
		t.Fatalf("expected newest session first, got %q", entries[0].Name)
	}

	if err := Rename("chat-one", "chat-renamed"); err != nil {
		t.Fatalf("rename session: %v", err)
	}
	if _, err := Load("chat-renamed"); err != nil {
		t.Fatalf("load renamed session: %v", err)
	}

	if err := Delete("chat-renamed"); err != nil {
		t.Fatalf("delete session: %v", err)
	}
	if _, err := os.Stat(mustPath(t, "chat-renamed")); !os.IsNotExist(err) {
		t.Fatalf("expected deleted session file to be gone, stat err=%v", err)
	}
}

func mustPath(t *testing.T, name string) string {
	t.Helper()
	path, err := Path(name)
	if err != nil {
		t.Fatalf("session path: %v", err)
	}
	return path
}
