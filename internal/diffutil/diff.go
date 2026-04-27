package diffutil

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func Unified(path, before, after string) string {
	tmpDir, err := os.MkdirTemp("", "gpterminal-diff-*")
	if err != nil {
		return fallback(path, before, after)
	}
	defer os.RemoveAll(tmpDir)

	oldPath := filepath.Join(tmpDir, "before")
	newPath := filepath.Join(tmpDir, "after")
	if err := os.WriteFile(oldPath, []byte(before), 0o600); err != nil {
		return fallback(path, before, after)
	}
	if err := os.WriteFile(newPath, []byte(after), 0o600); err != nil {
		return fallback(path, before, after)
	}

	cmd := exec.Command("git", "diff", "--no-index", "--no-color", "--", oldPath, newPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
			return fallback(path, before, after)
		}
	}

	diff := string(out)
	diff = strings.ReplaceAll(diff, oldPath, "a/"+path)
	diff = strings.ReplaceAll(diff, newPath, "b/"+path)
	diff = strings.ReplaceAll(diff, "--- a/"+path, "--- a/"+path)
	diff = strings.ReplaceAll(diff, "+++ b/"+path, "+++ b/"+path)
	if strings.TrimSpace(diff) == "" {
		return "No changes."
	}
	return diff
}

func fallback(path, before, after string) string {
	if before == after {
		return "No changes."
	}

	return fmt.Sprintf("--- a/%s\n+++ b/%s\n- %s\n+ %s\n", path, path, summarize(before), summarize(after))
}

func summarize(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "(empty)"
	}
	if len(s) > 200 {
		return s[:200] + "..."
	}
	return s
}
