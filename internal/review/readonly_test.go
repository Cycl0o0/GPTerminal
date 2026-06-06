package review

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// review is an invariant read-only command (INSTRUCTIONS.md §5): no disk writes,
// no git mutation, no command execution, no patch application. This guard scans
// the package source so a future edit cannot silently break that invariant.
func TestReadOnly_NoMutatingAPIsInSource(t *testing.T) {
	forbidden := []string{
		"os.WriteFile", "os.Create", "os.Remove", "os.Rename", "os.Mkdir",
		"os.OpenFile", "ioutil.WriteFile",
		"exec.Command", "exec.CommandContext", "syscall.Exec",
		"gitutil.Commit", "gitutil.Apply", "gitutil.Add", "gitutil.Checkout",
	}
	entries, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range entries {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, bad := range forbidden {
			if strings.Contains(string(src), bad) {
				t.Errorf("%s references forbidden mutating API %q — review must stay read-only", f, bad)
			}
		}
	}
}

// buildInput must not modify the file it reads.
func TestBuildInput_DoesNotMutateFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.go")
	content := "package x\nfunc Foo() {}\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	before, _ := os.Stat(path)

	out, err := buildInput(path, false, "")
	if err != nil {
		t.Fatalf("buildInput: %v", err)
	}
	if !strings.Contains(out, "Foo") {
		t.Fatalf("expected file content in prompt, got: %s", out)
	}

	after, _ := os.Stat(path)
	got, _ := os.ReadFile(path)
	if string(got) != content {
		t.Fatal("review modified the reviewed file")
	}
	if before.ModTime() != after.ModTime() {
		t.Fatal("review changed the file mtime")
	}
	// No stray files created in the dir.
	files, _ := os.ReadDir(dir)
	if len(files) != 1 {
		t.Fatalf("review created extra files: %d entries", len(files))
	}
}
