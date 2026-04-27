package gitutil

import (
	"fmt"
	"os/exec"
	"strings"
)

func runGit(args ...string) (string, error) {
	out, err := exec.Command("git", args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func IsRepo() bool {
	out, err := runGit("rev-parse", "--is-inside-work-tree")
	return err == nil && strings.TrimSpace(out) == "true"
}

func Diff(staged bool, paths ...string) (string, error) {
	args := []string{"diff", "--no-ext-diff"}
	if staged {
		args = append(args, "--staged")
	}
	if len(paths) > 0 {
		args = append(args, "--")
		args = append(args, paths...)
	}
	return runGit(args...)
}

func StatusShort() (string, error) {
	return runGit("status", "--short")
}

func RepoRoot() (string, error) {
	out, err := runGit("rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}
