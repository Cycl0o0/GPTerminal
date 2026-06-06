package eval

import (
	"encoding/json"
	"testing"

	"github.com/cycl0o0/GPTerminal/internal/execution"
)

// The baked-in suite must fully pass against the current policy. If a policy
// change breaks an expectation, this test (and `gpt eval`) fail loudly.
func TestDefaultCasesAllPass(t *testing.T) {
	rep := Run(DefaultCases())
	if !rep.OK {
		t.Fatalf("default eval suite failed: %d/%d\n%s", rep.Passed, rep.Total, rep.Text())
	}
	if rep.Total == 0 {
		t.Fatal("no cases in default suite")
	}
}

func TestRun_DetectsMismatch(t *testing.T) {
	// Deliberately wrong expectation -> must be reported as a failure.
	cases := []Case{{Name: "wrong", Command: "rm -rf /", Expect: execution.DecisionAllowed}}
	rep := Run(cases)
	if rep.OK || rep.Failed != 1 {
		t.Fatalf("expected 1 failure, got ok=%v failed=%d", rep.OK, rep.Failed)
	}
}

func TestParseFixture(t *testing.T) {
	raw := []byte(`[{"name":"a","command":"ls","expect":"allowed"}]`)
	cases, err := ParseFixture(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(cases) != 1 || cases[0].Command != "ls" {
		t.Fatalf("unexpected parse: %+v", cases)
	}
}

func TestParseFixture_RejectsEmptyCommand(t *testing.T) {
	if _, err := ParseFixture([]byte(`[{"name":"x","command":""}]`)); err == nil {
		t.Fatal("expected error for empty command")
	}
}

func TestReport_JSONValid(t *testing.T) {
	out, err := Run(DefaultCases()).JSON()
	if err != nil {
		t.Fatal(err)
	}
	var back Report
	if err := json.Unmarshal([]byte(out), &back); err != nil {
		t.Fatalf("eval JSON invalid: %v", err)
	}
}
