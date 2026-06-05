package aishell

import (
	"context"
	"testing"
)

// A user-typed command runs directly and its output is captured.
func TestExecuteInShell_UserTypedRuns(t *testing.T) {
	t.Setenv("GPTERMINAL_EXEC_POLICY", "1")
	code, out := executeInShell(context.Background(), "echo hi", t.TempDir(), "bash", false, nil)
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if out != "hi" {
		t.Fatalf("output = %q, want %q", out, "hi")
	}
}

// An LLM-generated Denied command must be refused and never executed, even
// though no Confirmer is wired (Denied bypasses confirmation entirely).
func TestExecuteInShell_GatedDeniedRefused(t *testing.T) {
	t.Setenv("GPTERMINAL_EXEC_POLICY", "1")
	code, out := executeInShell(context.Background(), "rm -rf /", t.TempDir(), "bash", true, nil)
	if code != 126 {
		t.Fatalf("exit = %d, want 126 (refused)", code)
	}
	if out != "" {
		t.Fatalf("output = %q, want empty", out)
	}
}
