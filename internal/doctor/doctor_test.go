package doctor

import (
	"encoding/json"
	"strings"
	"testing"
)

func find(checks []Check, name string) (Check, bool) {
	for _, c := range checks {
		if c.Name == name {
			return c, true
		}
	}
	return Check{}, false
}

func TestRun_ExecPolicyReflectsFlag(t *testing.T) {
	t.Setenv("GPTERMINAL_EXEC_POLICY", "0")
	rep := Run()
	c, ok := find(rep.Checks, "exec_policy")
	if !ok || c.Status != StatusWarn {
		t.Fatalf("exec_policy check = %+v, want warn when disabled", c)
	}

	t.Setenv("GPTERMINAL_EXEC_POLICY", "1")
	rep = Run()
	c, _ = find(rep.Checks, "exec_policy")
	if c.Status != StatusOK {
		t.Fatalf("exec_policy check = %+v, want ok when enabled", c)
	}
}

func TestReport_JSONStableAndValid(t *testing.T) {
	rep := Run()
	out, err := rep.JSON()
	if err != nil {
		t.Fatal(err)
	}
	var back Report
	if err := json.Unmarshal([]byte(out), &back); err != nil {
		t.Fatalf("doctor JSON is not valid: %v\n%s", err, out)
	}
	if len(back.Checks) == 0 {
		t.Fatal("expected at least one check in JSON")
	}
}

// The report must never leak a secret value, even if a check detail somehow
// contained one.
func TestReport_JSONRedacts(t *testing.T) {
	r := Report{OK: true, Checks: []Check{{Name: "x", Status: StatusOK, Detail: "leak sk-abcdefghij1234567890ABCD here"}}}
	out, _ := r.JSON()
	if strings.Contains(out, "sk-abcdefghij1234567890ABCD") {
		t.Fatalf("secret leaked into doctor JSON: %s", out)
	}
}
