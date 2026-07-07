package agent

import "testing"

func TestIsDone(t *testing.T) {
	cases := []struct {
		name string
		text string
		want bool
	}{
		{"marker on own line", "All set.\n[AGENT_DONE]\n", true},
		{"marker with summary suffix", "[AGENT_DONE] fixed the bug and ran tests", true},
		{"marker mid-sentence is not done", "I will emit [AGENT_DONE] when finished, but I am not done yet.", false},
		{"quoting the rule is not done", "The rules say to write `[AGENT_DONE]` at the end. Continuing now.", false},
		{"no marker", "Still working on step 2.", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isDone(tc.text); got != tc.want {
				t.Fatalf("isDone(%q) = %v, want %v", tc.text, got, tc.want)
			}
		})
	}
}
