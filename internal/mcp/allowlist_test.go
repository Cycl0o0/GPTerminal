package mcp

import (
	"errors"
	"testing"
)

func TestCheckAllowed_Unconfigured(t *testing.T) {
	t.Setenv("GPTERMINAL_MCP_ALLOWLIST", "")
	if err := checkAllowed("/usr/bin/anything"); err != nil {
		t.Fatalf("unconfigured allowlist should permit, got %v", err)
	}
}

func TestCheckAllowed_Enforced(t *testing.T) {
	t.Setenv("GPTERMINAL_MCP_ALLOWLIST", "npx, uvx")

	if err := checkAllowed("/usr/local/bin/npx"); err != nil {
		t.Fatalf("npx should be allowed: %v", err)
	}
	err := checkAllowed("/bin/bash")
	if !errors.Is(err, ErrNotAllowed) {
		t.Fatalf("bash should be refused, got %v", err)
	}
}
