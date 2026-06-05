package execution

import (
	"fmt"
	"regexp"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// secretEnvPattern matches environment variable names that look like secrets.
// Used to flag exfiltration like `echo $OPENAI_API_KEY` (INSTRUCTIONS.md §5).
var secretEnvPattern = regexp.MustCompile(`(?i)(API_KEY|SECRET|TOKEN|PASSWORD|PASSWD|CREDENTIAL|PRIVATE_KEY|ACCESS_KEY)`)

// secretFilePattern matches filenames that commonly hold secrets.
var secretFilePattern = regexp.MustCompile(`(?i)(^|/)(\.env(\.[\w.-]+)?|\.netrc|id_rsa|id_ed25519|\.pem|credentials|\.aws/credentials|\.ssh/)`)

// Classify applies the local, deterministic risk policy to a raw command
// string. It never consults an LLM (INSTRUCTIONS.md §9: no security-by-LLM).
//
// Complex shell (pipes, redirections, expansions) is *parsed*, not matched by
// regex over the raw string alone (INSTRUCTIONS.md §5).
func Classify(raw string) Verdict {
	v := Verdict{Decision: DecisionAllowed}

	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return v
	}

	// Fork-bomb signature is checked on the raw text: it parses as a function
	// definition, not a simple command, so AST walking alone would miss it.
	if strings.Contains(strings.ReplaceAll(raw, " ", ""), ":(){:|:&};:") {
		v.add(DecisionDenied, CategoryCatastrophic, "fork bomb")
	}

	parser := syntax.NewParser(syntax.KeepComments(false))
	file, err := parser.Parse(strings.NewReader(raw), "")
	if err != nil {
		// Fail-closed: unparseable shell cannot be reasoned about (§3 fail-closed).
		v.add(DecisionNeedsConfirm, CategoryUnparseable,
			fmt.Sprintf("unparseable shell, manual review required: %v", err))
		return v
	}

	syntax.Walk(file, func(node syntax.Node) bool {
		switch n := node.(type) {
		case *syntax.CallExpr:
			classifyCall(n, &v)
		case *syntax.BinaryCmd:
			classifyPipe(n, &v)
		case *syntax.Redirect:
			classifyRedirect(n, &v)
		case *syntax.ParamExp:
			if n.Param != nil && secretEnvPattern.MatchString(n.Param.Value) {
				v.add(DecisionNeedsConfirm, CategorySecretAccess,
					"references secret environment variable $"+n.Param.Value)
			}
		}
		return true
	})

	return v
}

// classifyCall inspects a single simple command (argv) for dangerous patterns.
func classifyCall(c *syntax.CallExpr, v *Verdict) {
	args := make([]string, 0, len(c.Args))
	for _, w := range c.Args {
		args = append(args, litWord(w))
	}
	if len(args) == 0 {
		return
	}
	cmd := baseName(args[0])
	rest := args[1:]
	flags, operands := splitArgs(rest)
	joined := strings.Join(args, " ")

	// mkfs has many filesystem-specific variants (mkfs.ext4, mkfs.xfs, ...).
	if strings.HasPrefix(cmd, "mkfs") {
		v.add(DecisionDenied, CategoryCatastrophic, "disk-formatting command: "+joined)
		return
	}

	switch cmd {
	case "sudo", "doas", "su":
		v.add(DecisionNeedsConfirm, CategoryPrivileged, "privilege escalation via "+cmd)
		// Re-classify the wrapped command.
		if len(operands) > 0 {
			classifyCall(&syntax.CallExpr{Args: c.Args[1:]}, v)
		}
		return

	case "rm":
		recursive := hasAnyFlag(flags, "r", "R", "recursive")
		force := hasAnyFlag(flags, "f", "force")
		for _, t := range operands {
			if isCatastrophicPath(t) {
				v.add(DecisionDenied, CategoryCatastrophic,
					"rm targeting critical path: "+t)
			}
		}
		if recursive {
			if force {
				v.add(DecisionNeedsConfirm, CategoryDestructive, "recursive force delete: "+joined)
			} else {
				v.add(DecisionNeedsConfirm, CategoryDestructive, "recursive delete: "+joined)
			}
		}

	case "git":
		// git clean -fdx / -ffdx wipes untracked + ignored files.
		if len(operands) > 0 && operands[0] == "clean" {
			f := strings.Join(flags, "")
			if strings.Contains(f, "f") && strings.Contains(f, "d") {
				v.add(DecisionNeedsConfirm, CategoryDestructive, "git clean removes untracked files: "+joined)
			}
		}

	case "find":
		if hasLongOperand(rest, "-delete") || hasLongOperand(rest, "-exec") && containsRm(rest) {
			v.add(DecisionNeedsConfirm, CategoryDestructive, "find deletes matched files: "+joined)
		}

	case "chmod", "chown":
		if hasAnyFlag(flags, "R", "recursive") {
			v.add(DecisionNeedsConfirm, CategoryDestructive, "recursive "+cmd+": "+joined)
		}

	case "fdisk", "parted":
		v.add(DecisionDenied, CategoryCatastrophic, "disk-formatting command: "+joined)

	case "dd":
		for _, op := range rest {
			if strings.HasPrefix(op, "of=/dev/") {
				v.add(DecisionDenied, CategoryCatastrophic, "dd writing to device: "+op)
			}
		}

	case "shutdown", "reboot", "halt", "poweroff", "init":
		v.add(DecisionDenied, CategoryCatastrophic, "system power/runlevel command: "+joined)

	case "cat", "less", "more", "head", "tail", "bat":
		for _, op := range operands {
			if secretFilePattern.MatchString(op) {
				v.add(DecisionNeedsConfirm, CategorySecretAccess, "reads secret file: "+op)
			}
		}

	}
}

// classifyPipe flags network-fetch piped directly into a shell interpreter,
// e.g. `curl http://x | sh` (INSTRUCTIONS.md §5).
func classifyPipe(b *syntax.BinaryCmd, v *Verdict) {
	if b.Op != syntax.Pipe && b.Op != syntax.PipeAll {
		return
	}
	left := firstCommandName(b.X)
	right := firstCommandName(b.Y)
	if isNetworkFetch(left) && isShellInterpreter(right) {
		v.add(DecisionDenied, CategoryNetworkToSh,
			fmt.Sprintf("piping %s into %s executes remote code", left, right))
	}
}

// classifyRedirect flags writes to block devices, e.g. `> /dev/sda`.
func classifyRedirect(r *syntax.Redirect, v *Verdict) {
	if r.Word == nil {
		return
	}
	target := litWord(r.Word)
	if strings.HasPrefix(target, "/dev/sd") || strings.HasPrefix(target, "/dev/nvme") ||
		strings.HasPrefix(target, "/dev/disk") || target == "/dev/mem" {
		v.add(DecisionDenied, CategoryCatastrophic, "redirect to block device: "+target)
	}
}

// ---- helpers ----------------------------------------------------------------

func isNetworkFetch(name string) bool {
	switch baseName(name) {
	case "curl", "wget", "fetch", "http", "https":
		return true
	}
	return false
}

func isShellInterpreter(name string) bool {
	switch baseName(name) {
	case "sh", "bash", "zsh", "dash", "ksh", "fish", "python", "python3", "perl", "ruby", "node":
		return true
	}
	return false
}

// isCatastrophicPath reports whether a delete target is system-critical: the
// filesystem root, the home dir, a wildcard root, or an absolute system path.
func isCatastrophicPath(p string) bool {
	p = strings.TrimSpace(p)
	switch p {
	case "/", "/*", "~", "~/", "$HOME", "${HOME}", "/*/", ".*", "*":
		return true
	}
	// Absolute paths into core system directories.
	for _, root := range []string{"/", "/bin", "/boot", "/etc", "/lib", "/sbin", "/usr", "/var", "/sys", "/proc", "/dev"} {
		if p == root || p == root+"/" || p == root+"/*" {
			return true
		}
	}
	return false
}

func baseName(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return p
}

// splitArgs separates flag tokens (-x, --long) from positional operands.
func splitArgs(args []string) (flags, operands []string) {
	for _, a := range args {
		if strings.HasPrefix(a, "-") && a != "-" && a != "--" {
			flags = append(flags, strings.TrimLeft(a, "-"))
		} else {
			operands = append(operands, a)
		}
	}
	return
}

// hasAnyFlag reports whether any short flag char or long flag name is present.
// Short flags are matched per-character so -rf contains both r and f.
func hasAnyFlag(flags []string, names ...string) bool {
	for _, f := range flags {
		for _, n := range names {
			if len(n) == 1 {
				if strings.Contains(f, n) {
					return true
				}
			} else if f == n {
				return true
			}
		}
	}
	return false
}

func hasLongOperand(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}

func containsRm(args []string) bool {
	for _, a := range args {
		if baseName(a) == "rm" {
			return true
		}
	}
	return false
}

// firstCommandName returns the program name of the first simple command found
// within a statement (descending into pipes/blocks).
func firstCommandName(stmt *syntax.Stmt) string {
	if stmt == nil {
		return ""
	}
	name := ""
	syntax.Walk(stmt, func(node syntax.Node) bool {
		if name != "" {
			return false
		}
		if c, ok := node.(*syntax.CallExpr); ok && len(c.Args) > 0 {
			name = litWord(c.Args[0])
			return false
		}
		return true
	})
	return name
}

// litWord renders a Word into a best-effort literal string for matching.
// Parameter expansions become $NAME so secret-name rules can still match.
func litWord(w *syntax.Word) string {
	if w == nil {
		return ""
	}
	var b strings.Builder
	for _, part := range w.Parts {
		switch p := part.(type) {
		case *syntax.Lit:
			b.WriteString(p.Value)
		case *syntax.SglQuoted:
			b.WriteString(p.Value)
		case *syntax.DblQuoted:
			for _, dp := range p.Parts {
				if lit, ok := dp.(*syntax.Lit); ok {
					b.WriteString(lit.Value)
				} else if pe, ok := dp.(*syntax.ParamExp); ok && pe.Param != nil {
					b.WriteString("$" + pe.Param.Value)
				}
			}
		case *syntax.ParamExp:
			if p.Param != nil {
				b.WriteString("$" + p.Param.Value)
			}
		}
	}
	return b.String()
}
