//go:build windows

package execution

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
)

// shellCommand builds the *exec.Cmd that runs raw through a shell on Windows.
//
// The flag passed to the interpreter depends on which interpreter is selected:
// cmd expects /C, sh/bash expect -c, and powershell expects -Command. Using the
// wrong flag (e.g. /C with bash) makes bash treat "/C" as a script path and
// fail with "/C: Is a directory". The interpreter defaults to cmd when r.Shell
// is empty.
func (r *Runner) shellCommand(ctx context.Context, raw string) *exec.Cmd {
	shell := r.Shell
	if shell == "" {
		shell = "cmd"
	}
	switch interpreterBase(shell) {
	case "bash", "sh", "zsh", "dash", "ash", "ksh", "mksh":
		return exec.CommandContext(ctx, shell, "-c", raw)
	case "powershell", "pwsh":
		return exec.CommandContext(ctx, shell, "-Command", raw)
	default: // cmd (and anything unrecognized) keeps /C
		return exec.CommandContext(ctx, shell, "/C", raw)
	}
}

// interpreterBase lowercases the interpreter name and strips any .exe suffix
// and directory so it can be matched regardless of how it was written.
func interpreterBase(shell string) string {
	base := strings.ToLower(filepath.Base(shell))
	return strings.TrimSuffix(base, ".exe")
}
