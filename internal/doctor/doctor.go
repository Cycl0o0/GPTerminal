// Package doctor runs offline, deterministic health checks on a GPTerminal
// install: provider/config presence, the execution-security posture, and tool
// availability. It never makes network calls and never prints secret values
// (INSTRUCTIONS.md §5 redaction, §8 stable JSON).
package doctor

import (
	"os"
	"os/exec" // used only for LookPath (PATH lookup); it executes nothing
	"path/filepath"

	"github.com/cycl0o0/GPTerminal/internal/config"
	"github.com/cycl0o0/GPTerminal/internal/execution"
)

// Status is a check outcome.
type Status string

const (
	StatusOK   Status = "ok"
	StatusWarn Status = "warn"
	StatusFail Status = "fail"
)

// Check is a single diagnostic result. Detail must never contain secret values.
type Check struct {
	Name   string `json:"name"`
	Status Status `json:"status"`
	Detail string `json:"detail"`
}

// Report is the full diagnostic output, designed for a stable --json contract.
type Report struct {
	OK     bool    `json:"ok"`
	Checks []Check `json:"checks"`
}

// providerSecretPresent reports whether the credential for the active provider
// is configured — WITHOUT returning the value.
func providerSecretPresent(provider string) bool {
	switch provider {
	case "anthropic":
		return config.AnthropicAPIKey() != ""
	case "gemini":
		return config.GeminiAPIKey() != ""
	case "openclaw":
		return config.OpenClawToken() != "" || config.OpenClawPassword() != ""
	default: // openai / compatible
		return config.APIKey() != "" || os.Getenv("OPENAI_API_KEY") != ""
	}
}

// Run executes all checks and returns the report. Pure aside from reading
// config/env and probing PATH; performs no network or shell execution.
func Run() Report {
	var checks []Check

	provider := config.ProviderName()
	if providerSecretPresent(provider) {
		checks = append(checks, Check{"provider", StatusOK, "provider=" + provider + ", credential configured"})
	} else {
		checks = append(checks, Check{"provider", StatusFail, "provider=" + provider + ", no credential configured"})
	}

	if m := config.Model(); m != "" {
		checks = append(checks, Check{"model", StatusOK, m})
	} else {
		checks = append(checks, Check{"model", StatusWarn, "no model set; defaults will apply"})
	}

	if f := config.ConfigFile(); fileExists(f) {
		checks = append(checks, Check{"config_file", StatusOK, f})
	} else {
		checks = append(checks, Check{"config_file", StatusWarn, "absent (" + f + "); run setup"})
	}

	// Security posture.
	if execution.PolicyEnabled() {
		checks = append(checks, Check{"exec_policy", StatusOK, "central execution policy enabled"})
	} else {
		checks = append(checks, Check{"exec_policy", StatusWarn, "GPTERMINAL_EXEC_POLICY disabled — commands bypass local policy"})
	}

	switch os.Getenv("GPTERMINAL_REDACT") {
	case "0", "false", "off", "no":
		checks = append(checks, Check{"redaction", StatusWarn, "GPTERMINAL_REDACT disabled — secrets not masked before LLM"})
	default:
		checks = append(checks, Check{"redaction", StatusOK, "secret redaction enabled"})
	}

	if al := os.Getenv("GPTERMINAL_MCP_ALLOWLIST"); al != "" {
		checks = append(checks, Check{"mcp_allowlist", StatusOK, "enforced: " + al})
	} else {
		checks = append(checks, Check{"mcp_allowlist", StatusWarn, "unset — any MCP server command may launch"})
	}

	if _, err := exec.LookPath("git"); err == nil {
		checks = append(checks, Check{"git", StatusOK, "found on PATH"})
	} else {
		checks = append(checks, Check{"git", StatusWarn, "not found on PATH; git features unavailable"})
	}

	if wd, err := os.Getwd(); err == nil {
		checks = append(checks, Check{"workspace", StatusOK, wd})
	}

	rep := Report{OK: true, Checks: checks}
	for _, c := range checks {
		if c.Status == StatusFail {
			rep.OK = false
		}
	}
	return rep
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

// ConfigDir is re-exported for callers that want to show where config lives.
func ConfigDir() string { return filepath.Clean(config.ConfigDir()) }
