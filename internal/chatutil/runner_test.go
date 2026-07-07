package chatutil

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	openai "github.com/sashabaranov/go-openai"
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

func TestParseSafeCommandHonorsQuotes(t *testing.T) {
	args, err := parseSafeCommand(`cat "a b.txt"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(args) != 2 || args[0] != "cat" || args[1] != "a b.txt" {
		t.Fatalf("expected [cat a b.txt], got %v", args)
	}

	args, err = parseSafeCommand(`git commit -m "fix: thing"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(args) != 4 || args[3] != "fix: thing" {
		t.Fatalf("expected message preserved as one arg, got %v", args)
	}

	// Single quotes keep their contents literal too.
	args, err = parseSafeCommand(`echo 'hello world'`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(args) != 2 || args[1] != "hello world" {
		t.Fatalf("expected [echo hello world], got %v", args)
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
	if _, err := r.readFile("secret-link.txt", 0, 0); err == nil {
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

func TestGlobToolMatchesPattern(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write a.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "b.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write b.txt: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatalf("mkdir sub: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "sub", "c.go"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write sub/c.go: %v", err)
	}

	r := &Runner{workDir: root}

	out, err := r.globFiles("*.go", "")
	if err != nil {
		t.Fatalf("glob *.go: %v", err)
	}
	if !contains(out, "a.go") || contains(out, "b.txt") {
		t.Fatalf("expected only a.go at top level, got %q", out)
	}

	out, err = r.globFiles("**/*.go", "")
	if err != nil {
		t.Fatalf("glob **/*.go: %v", err)
	}
	if !contains(out, "a.go") || !contains(out, "sub/c.go") {
		t.Fatalf("expected a.go and sub/c.go, got %q", out)
	}
}

func TestReadFileOffsetLimit(t *testing.T) {
	root := t.TempDir()
	content := "line1\nline2\nline3\nline4\nline5"
	if err := os.WriteFile(filepath.Join(root, "f.txt"), []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	r := &Runner{workDir: root}

	out, err := r.readFile("f.txt", 2, 2)
	if err != nil {
		t.Fatalf("readFile offset/limit: %v", err)
	}
	if !contains(out, "line2") || !contains(out, "line3") || contains(out, "line1") || contains(out, "line4") {
		t.Fatalf("expected only lines 2-3, got %q", out)
	}
	if !contains(out, "lines 2-3") {
		t.Fatalf("expected header to show line range, got %q", out)
	}
}

func TestMergeToolCallsKeepsDistinctIndexedCalls(t *testing.T) {
	// Reproduces the Anthropic/Gemini collapse bug: providers emit each tool
	// call as its own single-element event. Without a per-call Index they all
	// map to slice position 0 and collapse into one corrupted call. With Index
	// set (the fix), each stays separate.
	dest := map[int]*openai.ToolCall{}

	idx0, idx1 := 0, 1
	mergeToolCalls(dest, []openai.ToolCall{{
		Index:    &idx0,
		ID:       "call_a",
		Type:     openai.ToolTypeFunction,
		Function: openai.FunctionCall{Name: "read_file", Arguments: `{"path":"a.go"}`},
	}})
	mergeToolCalls(dest, []openai.ToolCall{{
		Index:    &idx1,
		ID:       "call_b",
		Type:     openai.ToolTypeFunction,
		Function: openai.FunctionCall{Name: "read_file", Arguments: `{"path":"b.go"}`},
	}})

	ordered := orderedToolCalls(dest)
	if len(ordered) != 2 {
		t.Fatalf("expected 2 distinct tool calls, got %d: %+v", len(ordered), ordered)
	}
	if ordered[0].ID != "call_a" || ordered[1].ID != "call_b" {
		t.Fatalf("tool calls collapsed or reordered: %+v", ordered)
	}
	if ordered[0].Function.Arguments != `{"path":"a.go"}` {
		t.Fatalf("first call args corrupted: %q", ordered[0].Function.Arguments)
	}
	if ordered[1].Function.Arguments != `{"path":"b.go"}` {
		t.Fatalf("second call args corrupted: %q", ordered[1].Function.Arguments)
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
