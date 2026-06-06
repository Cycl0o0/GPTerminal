package gptdo

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/cycl0o0/GPTerminal/internal/execution"
)

// In JSON mode, runCommands records a structured result per command: an allowed
// command runs (ran=true, exit_code set, output redacted); a denied command is
// recorded but never run (ran=false, no exit_code) and halts the step.
func TestRunCommands_JSONReport(t *testing.T) {
	t.Setenv("GPTERMINAL_EXEC_POLICY", "1")
	r := &runner{jsonMode: true, humanOut: io.Discard, cwd: t.TempDir()}

	cmds := []string{"echo sk-abcdefghij1234567890ABCD", "rm -rf /"}
	sr := &StepReport{Index: 1, Proposed: cmds}
	if _, err := r.runCommands(context.Background(), cmds, nil, sr); err != nil {
		t.Fatalf("runCommands: %v", err)
	}

	if len(sr.Commands) != 2 {
		t.Fatalf("recorded %d commands, want 2", len(sr.Commands))
	}

	allowed := sr.Commands[0]
	if allowed.Decision != execution.DecisionAllowed || !allowed.Ran || allowed.ExitCode == nil || *allowed.ExitCode != 0 {
		t.Fatalf("allowed cmd report unexpected: %+v", allowed)
	}
	if strings.Contains(allowed.Output, "sk-abcdefghij1234567890ABCD") {
		t.Fatalf("secret leaked into JSON output: %q", allowed.Output)
	}

	denied := sr.Commands[1]
	if denied.Decision != execution.DecisionDenied {
		t.Fatalf("second cmd decision = %s, want denied", denied.Decision)
	}
	if denied.Ran {
		t.Fatal("denied command was marked as ran")
	}
	if denied.ExitCode != nil {
		t.Fatalf("denied command has exit_code %v, want nil", *denied.ExitCode)
	}
}

func TestRunReport_JSONStableAndValid(t *testing.T) {
	exit := 0
	rep := RunReport{
		SchemaVersion: SchemaVersion,
		Request:       "do a thing",
		CWD:           "/repo",
		Steps: []StepReport{{
			Index:    1,
			Proposed: []string{"echo hi"},
			Commands: []CommandReport{{Command: "echo hi", Decision: execution.DecisionAllowed, Ran: true, ExitCode: &exit, Output: "hi\n"}},
		}},
		Completed: true,
	}
	b, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	var back RunReport
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatalf("gptdo report JSON invalid: %v", err)
	}
	if back.SchemaVersion != SchemaVersion || !back.Completed || len(back.Steps) != 1 {
		t.Fatalf("round-trip mismatch: %+v", back)
	}
}
