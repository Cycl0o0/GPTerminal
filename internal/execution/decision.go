// Package execution is the single, central path for running shell commands.
//
// Contract (INSTRUCTIONS.md §5): no other package may invoke os/exec, sh -c,
// or syscall.Exec for new code. Every command flows:
//
//	raw → parse (mvdan.cc/sh) → normalise → resolve cwd & symlinks
//	    → classify risk LOCALLY → preview → confirm if needed → run → capture
//
// The LLM may *propose* a command; only this deterministic, tested policy may
// *authorise* it. A Denied command is never executed, and --yes/AssumeYes only
// upgrades NeedsConfirm, never Denied.
package execution

import "errors"

// Decision is the local policy verdict for a command. It is the central
// invariant of the security model (INSTRUCTIONS.md §5).
type Decision string

const (
	// DecisionAllowed: safe to run without confirmation.
	DecisionAllowed Decision = "allowed"
	// DecisionNeedsConfirm: run only after explicit confirmation (or --yes).
	DecisionNeedsConfirm Decision = "needs_confirm"
	// DecisionDenied: never run. --yes does NOT override this.
	DecisionDenied Decision = "denied"
)

// Category labels why a command was classified, for previews and logs.
type Category string

const (
	CategorySafe          Category = "safe"
	CategoryDestructive   Category = "destructive"      // rm -rf ., find -delete, chmod -R 777
	CategoryCatastrophic  Category = "catastrophic"     // rm -rf /, mkfs, dd to device, fork bomb
	CategoryNetworkToSh   Category = "network_to_shell" // curl|sh, wget|sh
	CategorySecretAccess  Category = "secret_access"    // cat .env, echo $OPENAI_API_KEY
	CategoryWorkspaceEsc  Category = "workspace_escape"  // symlink / path outside workspace
	CategoryUnparseable   Category = "unparseable"
	CategoryPrivileged    Category = "privileged" // sudo, shutdown, reboot
)

// Sentinel errors so callers can branch with errors.Is (INSTRUCTIONS.md §6).
var (
	// ErrCommandDenied is returned when a command is Denied by policy.
	ErrCommandDenied = errors.New("command denied by local policy")
	// ErrConfirmationRequired is returned when confirmation is needed but no
	// confirmer is available (and AssumeYes is false).
	ErrConfirmationRequired = errors.New("command requires confirmation")
	// ErrConfirmationDeclined is returned when the user declined confirmation.
	ErrConfirmationDeclined = errors.New("command confirmation declined")
	// ErrWorkspaceEscape is returned when a path resolves outside the workspace.
	ErrWorkspaceEscape = errors.New("path escapes workspace boundary")
)

// Verdict is the full result of classifying a command.
type Verdict struct {
	Decision   Decision   `json:"decision"`
	Categories []Category `json:"categories,omitempty"`
	Reasons    []string   `json:"reasons,omitempty"`
}

// add records a reason and tightens the decision toward fail-closed. The
// ordering Allowed < NeedsConfirm < Denied means a verdict can only become
// stricter, never looser, as rules fire.
func (v *Verdict) add(d Decision, cat Category, reason string) {
	if rank(d) > rank(v.Decision) {
		v.Decision = d
	}
	if cat != "" {
		v.Categories = append(v.Categories, cat)
	}
	if reason != "" {
		v.Reasons = append(v.Reasons, reason)
	}
}

func rank(d Decision) int {
	switch d {
	case DecisionDenied:
		return 2
	case DecisionNeedsConfirm:
		return 1
	default:
		return 0
	}
}
