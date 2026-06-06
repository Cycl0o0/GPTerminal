package system

import (
	"strings"
	"testing"
)

// A denied command must not run through the legacy shim either; it is reported
// as a failed result (exit 126), never executed.
func TestExecuteCaptureInDir_DeniedNotRun(t *testing.T) {
	t.Setenv("GPTERMINAL_EXEC_POLICY", "1")
	res, err := ExecuteCaptureInDir("rm -rf /", t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success {
		t.Fatal("denied command reported success")
	}
	if res.ExitCode != 126 {
		t.Fatalf("exit = %d, want 126", res.ExitCode)
	}
	if !strings.Contains(res.Output, "refused by local policy") {
		t.Fatalf("output = %q, want policy-refusal note", res.Output)
	}
}

// An allowed command runs and its captured output is redacted.
func TestExecuteCaptureInDir_AllowedRunsAndRedacts(t *testing.T) {
	t.Setenv("GPTERMINAL_EXEC_POLICY", "1")
	res, err := ExecuteCaptureInDir("echo sk-abcdefghij1234567890ABCD", t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success || res.ExitCode != 0 {
		t.Fatalf("expected success, got success=%v exit=%d", res.Success, res.ExitCode)
	}
	if strings.Contains(res.Output, "sk-abcdefghij1234567890ABCD") {
		t.Fatalf("secret leaked in captured output: %q", res.Output)
	}
}
