//go:build windows

package execution

import (
	"context"
	"os/exec"
)

// shellCommand builds the *exec.Cmd that runs raw through a shell. On Windows
// this is `cmd /C <raw>` unless r.Shell overrides the interpreter.
func (r *Runner) shellCommand(ctx context.Context, raw string) *exec.Cmd {
	shell := r.Shell
	if shell == "" {
		shell = "cmd"
	}
	return exec.CommandContext(ctx, shell, "/C", raw)
}
