package execution

import "os"

// PolicyEnabled reports whether the central execution policy is active. It is ON
// by default (safe default, INSTRUCTIONS.md §3) and exists so the strangler-fig
// migration can be rolled back per environment without code changes
// (GPTERMINAL_EXEC_POLICY=0).
func PolicyEnabled() bool {
	switch os.Getenv("GPTERMINAL_EXEC_POLICY") {
	case "0", "false", "off", "no":
		return false
	}
	return true
}
