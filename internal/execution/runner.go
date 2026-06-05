package execution

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// Command is a request to run a shell command through the central runner.
type Command struct {
	Raw   string    // the raw command line
	Dir   string    // working directory (defaults to runner cwd / os.Getwd)
	Stdin io.Reader // optional stdin
	Env   []string  // optional environment (defaults to os.Environ)
}

// Result captures the outcome of a Run, including the policy verdict even when
// the command was not executed.
type Result struct {
	Command  string  `json:"command"`
	Verdict  Verdict `json:"verdict"`
	Ran      bool    `json:"ran"`
	ExitCode int     `json:"exit_code"`
	Stdout   string  `json:"stdout,omitempty"`
	Stderr   string  `json:"stderr,omitempty"`
}

// Confirmer is asked to approve a NeedsConfirm command. Returning (true, nil)
// approves; (false, nil) declines. It is never called for Denied commands.
type Confirmer func(ctx context.Context, cmd Command, v Verdict) (bool, error)

// Runner is the single, central path for executing shell commands. It is the
// ONLY place in the codebase permitted to call os/exec (INSTRUCTIONS.md §5).
type Runner struct {
	// Shell is the interpreter; defaults to $SHELL or "bash".
	Shell string
	// AssumeYes mirrors the --yes flag: it upgrades NeedsConfirm to execution
	// but NEVER converts Denied to allowed.
	AssumeYes bool
	// Confirm is consulted for NeedsConfirm commands when AssumeYes is false.
	Confirm Confirmer
	// Stdout/Stderr, when non-nil, stream output live; otherwise it is captured
	// into the Result.
	Stdout io.Writer
	Stderr io.Writer
	// Workspace, when set, bounds the working directory.
	Workspace *Workspace
}

// NewRunner returns a Runner with safe defaults: capture mode, fail-closed
// (no Confirmer means NeedsConfirm commands return ErrConfirmationRequired).
func NewRunner() *Runner {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "bash"
	}
	return &Runner{Shell: shell}
}

// Authorize runs the policy and confirmation gate without executing. It returns
// the verdict and, if the command may proceed, ok=true. Errors are the package
// sentinels (ErrCommandDenied, ErrConfirmationRequired, ErrConfirmationDeclined).
func (r *Runner) Authorize(ctx context.Context, cmd Command) (Verdict, bool, error) {
	v := Classify(cmd.Raw)

	switch v.Decision {
	case DecisionDenied:
		// Hard stop. AssumeYes does NOT override this (the central invariant).
		return v, false, fmt.Errorf("%w: %s", ErrCommandDenied, strings.Join(v.Reasons, "; "))

	case DecisionNeedsConfirm:
		if r.AssumeYes {
			return v, true, nil
		}
		if r.Confirm == nil {
			return v, false, ErrConfirmationRequired
		}
		ok, err := r.Confirm(ctx, cmd, v)
		if err != nil {
			return v, false, err
		}
		if !ok {
			return v, false, ErrConfirmationDeclined
		}
		return v, true, nil

	default: // DecisionAllowed
		return v, true, nil
	}
}

// Run authorizes and, if permitted, executes the command. A Result is always
// returned (even when blocked) so callers can render the verdict.
func (r *Runner) Run(ctx context.Context, cmd Command) (*Result, error) {
	v, ok, err := r.Authorize(ctx, cmd)
	res := &Result{Command: cmd.Raw, Verdict: v}
	if err != nil || !ok {
		return res, err
	}

	dir := cmd.Dir
	if dir == "" {
		dir, _ = os.Getwd()
	}
	if r.Workspace != nil && dir != "" {
		if _, werr := r.Workspace.ResolveWithin(dir, "."); werr != nil {
			return res, werr
		}
	}

	shell := r.Shell
	if shell == "" {
		shell = "bash"
	}

	c := exec.CommandContext(ctx, shell, "-c", cmd.Raw)
	c.Dir = dir
	if cmd.Env != nil {
		c.Env = cmd.Env
	}
	if cmd.Stdin != nil {
		c.Stdin = cmd.Stdin
	}

	var outBuf, errBuf bytes.Buffer
	if r.Stdout != nil {
		c.Stdout = r.Stdout
	} else {
		c.Stdout = &outBuf
	}
	if r.Stderr != nil {
		c.Stderr = r.Stderr
	} else {
		c.Stderr = &errBuf
	}

	runErr := c.Run()
	res.Ran = true
	res.Stdout = outBuf.String()
	res.Stderr = errBuf.String()

	if runErr == nil {
		res.ExitCode = 0
		return res, nil
	}
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		res.ExitCode = exitErr.ExitCode()
		return res, nil // non-zero exit is a normal result, not a Go error
	}
	res.ExitCode = -1
	return res, fmt.Errorf("execute %q: %w", cmd.Raw, runErr)
}
