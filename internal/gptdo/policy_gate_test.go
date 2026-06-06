package gptdo

import (
	"bufio"
	"strings"
	"testing"

	"github.com/cycl0o0/GPTerminal/internal/execution"
)

func newRunner(stdin string, autoApprove bool) *runner {
	return &runner{
		reader:      bufio.NewReader(strings.NewReader(stdin)),
		autoApprove: autoApprove,
	}
}

// autoApprove runs non-denied commands without a prompt (the --yes contract).
func TestApprove_AutoApproveRunsNonDenied(t *testing.T) {
	r := newRunner("", true)
	for _, d := range []execution.Decision{execution.DecisionAllowed, execution.DecisionNeedsConfirm} {
		ok, _, err := r.approve("x", execution.Verdict{Decision: d})
		if err != nil || !ok {
			t.Fatalf("autoApprove %s: ok=%v err=%v, want ok", d, ok, err)
		}
	}
}

// NeedsConfirm fails closed: default (empty input) declines.
func TestApprove_NeedsConfirmDefaultsNo(t *testing.T) {
	r := newRunner("\n", false)
	ok, _, err := r.approve("x", execution.Verdict{Decision: execution.DecisionNeedsConfirm})
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("NeedsConfirm with empty answer should decline (fail-closed)")
	}
}

func TestApprove_NeedsConfirmAcceptsYes(t *testing.T) {
	r := newRunner("y\n", false)
	ok, _, err := r.approve("x", execution.Verdict{Decision: execution.DecisionNeedsConfirm})
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v, want approved", ok, err)
	}
}

// Allowed defaults to Yes and supports [a]uto.
func TestApprove_AllowedAutoEnables(t *testing.T) {
	r := newRunner("a\n", false)
	ok, enableAuto, err := r.approve("x", execution.Verdict{Decision: execution.DecisionAllowed})
	if err != nil || !ok || !enableAuto {
		t.Fatalf("ok=%v auto=%v err=%v, want approved+auto", ok, enableAuto, err)
	}
}

func TestFormatDeniedCommand_TellsModelNotToRetry(t *testing.T) {
	out := formatDeniedCommand("rm -rf /", execution.Verdict{
		Decision: execution.DecisionDenied,
		Reasons:  []string{"rm targeting critical path: /"},
	})
	if !strings.Contains(out, "DENIED") || !strings.Contains(out, "Do not retry") {
		t.Fatalf("denied report missing guidance:\n%s", out)
	}
}
