package mcp

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrNotAllowed is returned when an MCP server command is not on the allowlist.
var ErrNotAllowed = fmt.Errorf("mcp: server command not allowed")

// allowlist returns the set of permitted MCP server command base-names, parsed
// from GPTERMINAL_MCP_ALLOWLIST (comma-separated). An empty result means the
// allowlist is unconfigured.
//
// Policy (INSTRUCTIONS.md §5 "Allowlist MCP"): when the allowlist IS configured
// it is enforced fail-closed — any command not listed is refused. When it is
// unconfigured the launcher allows the command but the caller is expected to
// surface that an allowlist is recommended; we keep the permissive default only
// to avoid breaking existing user configs in a patch release.
func allowlist() map[string]bool {
	raw := strings.TrimSpace(os.Getenv("GPTERMINAL_MCP_ALLOWLIST"))
	if raw == "" {
		return nil
	}
	set := make(map[string]bool)
	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			set[filepath.Base(item)] = true
		}
	}
	return set
}

// checkAllowed enforces the allowlist for a server command. It returns nil when
// the command is permitted (or when no allowlist is configured).
func checkAllowed(command string) error {
	set := allowlist()
	if set == nil {
		return nil
	}
	base := filepath.Base(command)
	if !set[base] {
		return fmt.Errorf("%w: %q (allow it via GPTERMINAL_MCP_ALLOWLIST)", ErrNotAllowed, base)
	}
	return nil
}
