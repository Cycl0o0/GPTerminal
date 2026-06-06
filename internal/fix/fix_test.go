package fix

import (
	"context"
	"testing"
)

// fix must never rerun a non-allowed (potentially destructive) command just to
// capture its error output.
func TestCaptureError_SkipsNonAllowedCommand(t *testing.T) {
	t.Setenv("GPTERMINAL_EXEC_POLICY", "1")
	if got := captureError(context.Background(), "rm -rf /"); got != "" {
		t.Fatalf("captureError reran a denied command, got %q", got)
	}
}

// An allowed command is rerun and its output captured.
func TestCaptureError_RunsAllowedCommand(t *testing.T) {
	t.Setenv("GPTERMINAL_EXEC_POLICY", "1")
	if got := captureError(context.Background(), "echo hi"); got != "hi" {
		t.Fatalf("captureError = %q, want %q", got, "hi")
	}
}
