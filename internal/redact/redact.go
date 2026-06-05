// Package redact removes secrets from text before it leaves the process —
// before any LLM request AND before logs, TUI output, JSON, or error messages
// (INSTRUCTIONS.md §5, §8, §9). Redaction is fail-safe: it never errors, it only
// masks. Callers must apply it at every egress boundary.
package redact

import (
	"os"
	"regexp"
	"sort"
	"strings"
)

// Mask is the placeholder substituted for any detected secret.
const Mask = "«REDACTED»"

// secretEnvNames matches environment variable names whose *values* are secrets.
var secretEnvNames = regexp.MustCompile(`(?i)(API_KEY|_KEY|SECRET|TOKEN|PASSWORD|PASSWD|CREDENTIAL|ACCESS_KEY|SESSION_KEY|AUTH)`)

// patterns match secret-shaped tokens by structure, independent of any env.
var patterns = []*regexp.Regexp{
	regexp.MustCompile(`sk-[A-Za-z0-9_-]{16,}`),                   // OpenAI-style
	regexp.MustCompile(`sk-ant-[A-Za-z0-9_-]{16,}`),              // Anthropic
	regexp.MustCompile(`AIza[A-Za-z0-9_-]{20,}`),                 // Google API key
	regexp.MustCompile(`gh[pousr]_[A-Za-z0-9]{20,}`),            // GitHub tokens
	regexp.MustCompile(`xox[baprs]-[A-Za-z0-9-]{10,}`),         // Slack
	regexp.MustCompile(`AKIA[0-9A-Z]{16}`),                       // AWS access key id
	regexp.MustCompile(`(?i)\bbearer\s+[A-Za-z0-9._\-]{12,}`),   // Authorization: Bearer
	regexp.MustCompile(`eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{6,}`), // JWT
	regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----[\s\S]*?-----END [A-Z ]*PRIVATE KEY-----`),
}

// assignment catches KEY=value / "KEY": "value" style leaks where KEY looks
// secret. Group 1 is the leading name+separator we keep; group 2 is the value.
var assignment = regexp.MustCompile(`(?i)([A-Z0-9_]*(?:API_KEY|SECRET|TOKEN|PASSWORD|PASSWD|CREDENTIAL|ACCESS_KEY|PRIVATE_KEY|AUTH)[A-Z0-9_]*\s*[:=]\s*["']?)([^\s"',]+)`)

// String returns s with secrets masked. It is safe to call on any text.
func String(s string) string {
	if s == "" {
		return s
	}
	out := s

	// 1) Mask exact secret values pulled from the current environment. This is
	//    the strongest signal: if the literal token is present, redact it.
	for _, v := range secretEnvValues() {
		if v != "" {
			out = strings.ReplaceAll(out, v, Mask)
		}
	}

	// 2) Mask structural secret tokens.
	for _, re := range patterns {
		out = re.ReplaceAllString(out, Mask)
	}

	// 3) Mask secret-named assignments, preserving the key for readability.
	out = assignment.ReplaceAllString(out, "${1}"+Mask)

	return out
}

// Strings redacts a slice in place-style (returns a new slice).
func Strings(in []string) []string {
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = String(s)
	}
	return out
}

// Error redacts an error's message. Returns nil for nil.
func Error(err error) string {
	if err == nil {
		return ""
	}
	return String(err.Error())
}

// secretEnvValues collects values of environment variables whose names look
// like secrets, longest-first so substring values are masked before prefixes.
func secretEnvValues() []string {
	var vals []string
	for _, kv := range os.Environ() {
		eq := strings.IndexByte(kv, '=')
		if eq <= 0 {
			continue
		}
		name, val := kv[:eq], kv[eq+1:]
		if len(val) >= 6 && secretEnvNames.MatchString(name) {
			vals = append(vals, val)
		}
	}
	sort.Slice(vals, func(i, j int) bool { return len(vals[i]) > len(vals[j]) })
	return vals
}
