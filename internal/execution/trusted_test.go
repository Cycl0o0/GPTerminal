package execution

import (
	"context"
	"strings"
	"testing"

	"github.com/cycl0o0/GPTerminal/internal/redact"
)

// Trusted execution bypasses classification (authorize only — no exec here, so
// using a Denied command string is safe).
func TestRunner_TrustedAuthorizeAllows(t *testing.T) {
	t.Setenv("GPTERMINAL_EXEC_POLICY", "1")
	r := NewRunner()
	r.Trusted = true
	v, ok, err := r.Authorize(context.Background(), Command{Raw: "rm -rf /"})
	if err != nil || !ok || v.Decision != DecisionAllowed {
		t.Fatalf("trusted authorize = (%s, ok=%v, err=%v), want allowed/ok", v.Decision, ok, err)
	}
}

// The GPTERMINAL_EXEC_POLICY rollback flag disables classification globally.
func TestRunner_PolicyDisabledAuthorizeAllows(t *testing.T) {
	t.Setenv("GPTERMINAL_EXEC_POLICY", "0")
	r := NewRunner()
	v, ok, err := r.Authorize(context.Background(), Command{Raw: "rm -rf /"})
	if err != nil || !ok || v.Decision != DecisionAllowed {
		t.Fatalf("policy-disabled authorize = (%s, ok=%v, err=%v), want allowed/ok", v.Decision, ok, err)
	}
}

// RedactOutput masks secrets in captured output (which is often fed to an LLM).
func TestRunner_RedactOutputMasksCapturedSecret(t *testing.T) {
	t.Setenv("GPTERMINAL_EXEC_POLICY", "1")
	r := NewRunner()
	r.Shell = "bash"
	r.RedactOutput = true
	res, err := r.Run(context.Background(), Command{Raw: "echo sk-abcdefghij1234567890ABCD"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if strings.Contains(res.Stdout, "sk-abcdefghij1234567890ABCD") {
		t.Fatalf("secret leaked in captured stdout: %q", res.Stdout)
	}
	if !strings.Contains(res.Stdout, redact.Mask) {
		t.Fatalf("expected mask in stdout, got %q", res.Stdout)
	}
}
