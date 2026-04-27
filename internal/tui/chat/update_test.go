package chat

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cycl0o0/GPTerminal/internal/chatutil"
	"github.com/cycl0o0/GPTerminal/internal/session"
	"github.com/cycl0o0/GPTerminal/internal/system"
)

func TestExitInputQuitsChat(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	m := NewModel(nil, system.SystemInfo{}, Options{})
	m.textarea.SetValue(" exit ")

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected quit command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("expected quit message, got %T", cmd())
	}

	got, ok := updated.(Model)
	if !ok {
		t.Fatalf("expected Model, got %T", updated)
	}
	if got.streaming {
		t.Fatal("exit input should not start streaming")
	}
	if len(got.messages) != 0 {
		t.Fatalf("exit input should not be added to history, got %d messages", len(got.messages))
	}
}

func TestCtrlSSavesSession(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	m := NewModel(nil, system.SystemInfo{}, Options{})
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	if cmd != nil {
		_ = cmd()
	}

	got := updated.(Model)
	if got.sessionName == "" {
		t.Fatal("expected Ctrl+S to assign a session name")
	}
	if _, err := session.Load(got.sessionName); err != nil {
		t.Fatalf("expected saved session to exist: %v", err)
	}
}

func TestCtrlXCancelsCurrentStream(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	canceled := false
	m := NewModel(nil, system.SystemInfo{}, Options{})
	m.streaming = true
	m.streamCancel = func() { canceled = true }
	m.approvalCh = make(chan chatutil.ApprovalDecision)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	got := updated.(Model)

	if !canceled {
		t.Fatal("expected Ctrl+X to cancel the current stream")
	}
	if got.notice == "" {
		t.Fatal("expected cancel notice to be set")
	}
}
