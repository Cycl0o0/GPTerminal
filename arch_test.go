package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// shellStringExec matches a literal shell-string invocation such as
// exec.Command("bash", "-c", cmd) or exec.CommandContext(ctx, "sh", "-lc", cmd).
// It deliberately requires a *quoted* shell name and flag, so the sanctioned
// runner in internal/execution — which uses a variable shell (exec.Command(ctx,
// shell, "-c", raw)) — does not match, and fixed-argv calls like
// exec.Command(name, args...) or exec.Command("git", "diff") do not match either.
var shellStringExec = regexp.MustCompile(
	`exec\.Command(?:Context)?\(\s*(?:[\w.]+\s*,\s*)?"(?:bash|sh|zsh|dash|ksh|fish|cmd)"\s*,\s*"(?:-c|-lc|-lic|/[Cc])"`,
)

// TestNoRawShellStringExecOutsideExecutionPackage enforces the central
// invariant of v3.0.0/v3.1.0 (INSTRUCTIONS.md §5): every dynamic shell-string
// command must run through internal/execution.Runner. Raw `exec.Command("bash",
// "-c", ...)` style execution anywhere else fails the build.
//
// This is more valuable than a coverage target — it protects the exact property
// the security model claims.
func TestNoRawShellStringExecOutsideExecutionPackage(t *testing.T) {
	root := moduleRoot(t)

	// Only the central execution package may hold shell-invocation code.
	allowedDir := filepath.Join("internal", "execution")

	var violations []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			base := d.Name()
			if base == "vendor" || base == ".git" || base == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		if strings.HasPrefix(rel, allowedDir+string(filepath.Separator)) {
			return nil
		}

		src, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		for i, line := range strings.Split(string(src), "\n") {
			if shellStringExec.MatchString(line) {
				violations = append(violations, rel+":"+itoa(i+1)+"  "+strings.TrimSpace(line))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(violations) > 0 {
		t.Fatalf("raw shell-string execution found outside internal/execution "+
			"(route it through execution.Runner):\n  %s", strings.Join(violations, "\n  "))
	}
}

// TestShellStringExecRegexHasTeeth guards the guard: the pattern must catch real
// shell-string execution and must NOT flag the sanctioned variable-shell runner
// or fixed-argv calls.
func TestShellStringExecRegexHasTeeth(t *testing.T) {
	mustMatch := []string{
		`exec.Command("bash","-c",c)`,
		`exec.CommandContext(ctx, "sh", "-c", x)`,
		`exec.Command("bash", "-lc", command)`,
		`exec.Command("cmd", "/C", command)`,
	}
	mustNotMatch := []string{
		`exec.CommandContext(ctx, shell, "-c", raw)`, // sanctioned runner (variable shell)
		`exec.Command("git", "diff")`,                // fixed argv
		`exec.Command(name, args...)`,                // variadic argv
	}
	for _, s := range mustMatch {
		if !shellStringExec.MatchString(s) {
			t.Errorf("regex should match shell-string exec: %s", s)
		}
	}
	for _, s := range mustNotMatch {
		if shellStringExec.MatchString(s) {
			t.Errorf("regex should NOT match: %s", s)
		}
	}
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found walking up from cwd")
		}
		dir = parent
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
