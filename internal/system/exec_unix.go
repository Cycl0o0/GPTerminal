//go:build !windows

package system

import (
	"errors"
	"os"
	"os/exec"
)

func Execute(command string) error {
	cmd := exec.Command("bash", "-c", command)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func ExecuteCapture(command string) (ExecResult, error) {
	return ExecuteCaptureInDir(command, "")
}

func ExecuteCaptureInDir(command, dir string) (ExecResult, error) {
	cmd := exec.Command("bash", "-lc", command)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()

	result := ExecResult{
		Output:  string(out),
		Success: err == nil,
	}

	if err == nil {
		return result, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
		return result, nil
	}

	result.ExitCode = -1
	if result.Output == "" {
		result.Output = err.Error()
	}
	return result, err
}
