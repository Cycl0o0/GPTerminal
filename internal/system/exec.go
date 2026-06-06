package system

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/cycl0o0/GPTerminal/internal/execution"
)

// This file is the compatibility shim that keeps the historical system.Execute*
// API while routing ALL shell-string execution through the central
// execution.Runner (INSTRUCTIONS.md §5, v3.1.0 unified-execution). Callers that
// pass dynamic/LLM-derived command strings are now subject to the local policy:
// a Denied command is never run, even here. Confirmation is auto-approved
// (AssumeYes) because the historical callers do their own interactive prompts;
// Denied still cannot be bypassed.

// Execute runs a command interactively (stdio attached to the terminal). It
// returns execution.ErrCommandDenied if the command is blocked by policy.
func Execute(command string) error {
	r := execution.NewRunner()
	r.AssumeYes = true
	r.Stdout = os.Stdout
	r.Stderr = os.Stderr

	_, err := r.Run(context.Background(), execution.Command{
		Raw:   command,
		Stdin: os.Stdin,
	})
	return err
}

// ExecuteCapture runs a command and captures its combined output. Output is
// redacted (it is commonly fed back to an LLM).
func ExecuteCapture(command string) (ExecResult, error) {
	return ExecuteCaptureInDir(command, "")
}

// ExecuteCaptureInDir runs a command in dir and captures its combined output.
// A Denied command is reported as a failed result (exit 126) rather than run.
func ExecuteCaptureInDir(command, dir string) (ExecResult, error) {
	r := execution.NewRunner()
	r.AssumeYes = true
	r.RedactOutput = true // captured output is often sent to the model

	res, err := r.Run(context.Background(), execution.Command{
		Raw: command,
		Dir: dir,
	})

	// Policy refusal: surface as a non-success result, not a Go error, so the
	// historical callers keep working (they branch on ExecResult.Success).
	if errors.Is(err, execution.ErrCommandDenied) {
		return ExecResult{
			Output:   fmt.Sprintf("refused by local policy: %s", joinReasons(res)),
			ExitCode: 126,
			Success:  false,
		}, nil
	}

	out := ""
	if res != nil {
		out = res.Stdout + res.Stderr
	}
	result := ExecResult{
		Output:   out,
		ExitCode: -1,
		Success:  err == nil && res != nil && res.Ran && res.ExitCode == 0,
	}
	if res != nil {
		result.ExitCode = res.ExitCode
	}
	if err != nil && !res.Ran {
		// Genuine execution failure (not a non-zero exit).
		if result.Output == "" {
			result.Output = err.Error()
		}
		return result, err
	}
	return result, nil
}

func joinReasons(res *execution.Result) string {
	if res == nil || len(res.Verdict.Reasons) == 0 {
		return "denied"
	}
	out := ""
	for i, r := range res.Verdict.Reasons {
		if i > 0 {
			out += "; "
		}
		out += r
	}
	return out
}
