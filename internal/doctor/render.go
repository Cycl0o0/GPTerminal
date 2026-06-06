package doctor

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cycl0o0/GPTerminal/internal/redact"
)

// JSON renders the report as a stable, indented JSON document. Every detail is
// passed through redaction as defense-in-depth before egress (INSTRUCTIONS.md §8).
func (r Report) JSON() (string, error) {
	safe := r
	safe.Checks = make([]Check, len(r.Checks))
	for i, c := range r.Checks {
		c.Detail = redact.String(c.Detail)
		safe.Checks[i] = c
	}
	b, err := json.MarshalIndent(safe, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal doctor report: %w", err)
	}
	return string(b), nil
}

// Text renders a human-readable report with status glyphs.
func (r Report) Text() string {
	var b strings.Builder
	b.WriteString("GPTerminal doctor\n")
	for _, c := range r.Checks {
		b.WriteString(fmt.Sprintf("  %s  %-14s %s\n", glyph(c.Status), c.Name, redact.String(c.Detail)))
	}
	if r.OK {
		b.WriteString("\nAll required checks passed.\n")
	} else {
		b.WriteString("\nOne or more required checks FAILED.\n")
	}
	return b.String()
}

func glyph(s Status) string {
	switch s {
	case StatusOK:
		return "✓"
	case StatusWarn:
		return "!"
	default:
		return "✗"
	}
}
