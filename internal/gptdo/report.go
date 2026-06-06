package gptdo

import "github.com/cycl0o0/GPTerminal/internal/execution"

// SchemaVersion is the stable contract version for `gptdo --json` output.
// Increment only for breaking changes; add fields additively within a version.
const SchemaVersion = 1

// CommandReport is the machine-readable outcome of a single proposed command.
// ExitCode is a pointer so `ran:false` (denied/declined) has no exit code rather
// than an ambiguous -1.
type CommandReport struct {
	Command  string             `json:"command"`
	Decision execution.Decision `json:"decision"`
	Reasons  []string           `json:"reasons,omitempty"`
	Ran      bool               `json:"ran"`
	ExitCode *int               `json:"exit_code,omitempty"`
	Output   string             `json:"output,omitempty"` // combined stdout+stderr, redacted
}

// StepReport is one planning step and the commands it ran.
type StepReport struct {
	Index    int             `json:"index"`
	Message  string          `json:"message,omitempty"`
	Proposed []string        `json:"proposed_commands"`
	Commands []CommandReport `json:"commands"`
}

// RunReport is the top-level `gptdo --json` document.
type RunReport struct {
	SchemaVersion int          `json:"schema_version"`
	Request       string       `json:"request"`
	CWD           string       `json:"cwd"`
	Steps         []StepReport `json:"steps"`
	Completed     bool         `json:"completed"`
	Aborted       bool         `json:"aborted"`
	Summary       string       `json:"summary,omitempty"`
	Error         *string      `json:"error,omitempty"`
}
