package execution

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestRunner_AllowedExecutes(t *testing.T) {
	r := NewRunner()
	r.Shell = "bash"
	res, err := r.Run(context.Background(), Command{Raw: "echo hi"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !res.Ran || res.ExitCode != 0 {
		t.Fatalf("ran=%v exit=%d", res.Ran, res.ExitCode)
	}
	if got := res.Stdout; got != "hi\n" {
		t.Fatalf("stdout = %q, want %q", got, "hi\n")
	}
}

// Denied must never run, even with AssumeYes (the central invariant, §5).
func TestRunner_DeniedNotExecutedEvenWithYes(t *testing.T) {
	r := NewRunner()
	r.AssumeYes = true
	r.Confirm = func(context.Context, Command, Verdict) (bool, error) {
		t.Fatal("confirmer must not be consulted for Denied")
		return true, nil
	}
	res, err := r.Run(context.Background(), Command{Raw: "rm -rf /"})
	if !errors.Is(err, ErrCommandDenied) {
		t.Fatalf("err = %v, want ErrCommandDenied", err)
	}
	if res.Ran {
		t.Fatal("Denied command was executed")
	}
}

// AssumeYes upgrades NeedsConfirm (only) to execution.
func TestRunner_AssumeYesUpgradesNeedsConfirm(t *testing.T) {
	dir := t.TempDir()
	r := NewRunner()
	r.Shell = "bash"
	r.AssumeYes = true
	// `chmod -R` is NeedsConfirm; run it harmlessly inside a temp dir.
	res, err := r.Run(context.Background(), Command{Raw: "chmod -R 755 .", Dir: dir})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !res.Ran {
		t.Fatal("NeedsConfirm not executed under AssumeYes")
	}
	if res.Verdict.Decision != DecisionNeedsConfirm {
		t.Fatalf("verdict = %s, want needs_confirm", res.Verdict.Decision)
	}
}

// Without AssumeYes and without a Confirmer, NeedsConfirm fails closed.
func TestRunner_NeedsConfirmFailsClosed(t *testing.T) {
	r := NewRunner()
	res, err := r.Run(context.Background(), Command{Raw: "git clean -fdx"})
	if !errors.Is(err, ErrConfirmationRequired) {
		t.Fatalf("err = %v, want ErrConfirmationRequired", err)
	}
	if res.Ran {
		t.Fatal("NeedsConfirm executed without confirmation")
	}
}

func TestRunner_DeclinedConfirmation(t *testing.T) {
	r := NewRunner()
	r.Confirm = func(context.Context, Command, Verdict) (bool, error) { return false, nil }
	_, err := r.Run(context.Background(), Command{Raw: "git clean -fdx"})
	if !errors.Is(err, ErrConfirmationDeclined) {
		t.Fatalf("err = %v, want ErrConfirmationDeclined", err)
	}
}

// Symlink-escape evasion: `ln -s /etc/passwd local && cat local`. The workspace
// resolver must refuse to resolve `local` because it points outside the root.
func TestWorkspace_SymlinkEscapeBlocked(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink semantics differ on windows")
	}
	root := t.TempDir()
	ws, err := NewWorkspace(root)
	if err != nil {
		t.Fatal(err)
	}

	// Pick a target that exists and is outside the workspace.
	outside := "/etc/hostname"
	if _, err := os.Stat(outside); err != nil {
		outside = os.TempDir() // fallback; still outside `root`
	}
	link := filepath.Join(root, "local")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}

	if _, err := ws.ResolveWithin(root, "local"); !errors.Is(err, ErrWorkspaceEscape) {
		t.Fatalf("ResolveWithin escaped symlink err = %v, want ErrWorkspaceEscape", err)
	}

	// A normal in-workspace path resolves fine.
	inside := filepath.Join(root, "ok.txt")
	if err := os.WriteFile(inside, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ws.ResolveWithin(root, "ok.txt"); err != nil {
		t.Fatalf("in-workspace path rejected: %v", err)
	}
}

func TestWorkspace_NonexistentEscapeBlocked(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip()
	}
	root := t.TempDir()
	ws, _ := NewWorkspace(root)
	// A path climbing out with .. must be refused even though it doesn't exist.
	if _, err := ws.ResolveWithin(root, "../../etc/shadow"); !errors.Is(err, ErrWorkspaceEscape) {
		t.Fatalf("err = %v, want ErrWorkspaceEscape", err)
	}
}
