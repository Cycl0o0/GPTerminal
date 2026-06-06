//go:build !windows

package execution

import (
	"context"
	"os"
	"os/exec"
)

// shellCommand builds the *exec.Cmd that runs raw through a shell. On Unix this
// is `<shell> -c <raw>`, where shell is r.Shell, then $SHELL, then bash.
func (r *Runner) shellCommand(ctx context.Context, raw string) *exec.Cmd {
	shell := r.Shell
	if shell == "" {
		shell = os.Getenv("SHELL")
	}
	if shell == "" {
		shell = "bash"
	}
	return exec.CommandContext(ctx, shell, "-c", raw)
}
