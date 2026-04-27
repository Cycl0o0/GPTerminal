package chatutil

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestBuildUserMessage(t *testing.T) {
	got := BuildUserMessage("summarize this", "line 1\nline 2")
	if got == "" {
		t.Fatal("expected non-empty message")
	}
	if want := "Piped stdin:"; !contains(got, want) {
		t.Fatalf("expected %q in message, got %q", want, got)
	}
}

func TestParseSafeCommandRejectsShellOperators(t *testing.T) {
	if _, err := parseSafeCommand("git status | cat"); err == nil {
		t.Fatal("expected shell operator rejection")
	}
}

func TestValidateSafeCommand(t *testing.T) {
	if err := validateSafeCommand([]string{"git", "status"}); err != nil {
		t.Fatalf("expected git status to be allowed: %v", err)
	}
	if err := validateSafeCommand([]string{"rm", "-rf", "/"}); err == nil {
		t.Fatal("expected rm to be rejected")
	}
}

func TestValidateWritableCommand(t *testing.T) {
	if err := validateWritableCommand([]string{"mkdir", "tmp"}); err != nil {
		t.Fatalf("expected mkdir to be allowed in write mode: %v", err)
	}
	if err := validateWritableCommand([]string{"curl", "https://example.com"}); err == nil {
		t.Fatal("expected curl to be rejected in chat tool write mode")
	}
}

func TestResolveWorkspacePathRejectsEscape(t *testing.T) {
	root := t.TempDir()
	r := &Runner{workDir: root}

	if _, err := r.resolveWorkspacePath("../outside.txt"); err == nil {
		t.Fatal("expected parent traversal to be rejected")
	}
}

func TestReadFileRejectsSymlinkOutsideWorkspace(t *testing.T) {
	root := t.TempDir()
	outsideDir := t.TempDir()
	outsideFile := filepath.Join(outsideDir, "secret.txt")
	if err := os.WriteFile(outsideFile, []byte("secret"), 0o644); err != nil {
		t.Fatalf("write outside file: %v", err)
	}

	linkPath := filepath.Join(root, "secret-link.txt")
	if err := os.Symlink(outsideFile, linkPath); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	r := &Runner{workDir: root}
	if _, err := r.readFile("secret-link.txt"); err == nil {
		t.Fatal("expected symlink escaping workspace to be rejected")
	}
}

func TestValidateCommandArgsRejectsAbsolutePath(t *testing.T) {
	root := t.TempDir()
	r := &Runner{workDir: root}

	absPath := "/etc/passwd"
	if runtime.GOOS == "windows" {
		absPath = `C:\Windows\System32\config`
	}

	if err := r.validateCommandArgs([]string{"cat", absPath}); err == nil {
		t.Fatal("expected absolute path to be rejected")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || index(s, substr) >= 0)
}

func index(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
