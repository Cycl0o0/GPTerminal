package redact

import (
	"strings"
	"testing"
)

func TestString_StructuralTokens(t *testing.T) {
	cases := []struct {
		name, in string
	}{
		{"openai", "key is sk-abcdefghij1234567890ABCDEF here"},
		{"anthropic", "sk-ant-api03-abcdefghij1234567890"},
		{"google", "AIzaSyA1234567890abcdefghijklmnopqrstuv"},
		{"github", "ghp_abcdefghijklmnopqrstuvwxyz0123456789"},
		{"aws", "AKIAIOSFODNN7EXAMPLE"},
		{"bearer", "Authorization: Bearer abcdef123456.ghijkl"},
		{"jwt", "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := String(tc.in)
			if !strings.Contains(got, Mask) {
				t.Fatalf("expected mask in %q", got)
			}
		})
	}
}

func TestString_Assignments(t *testing.T) {
	in := `OPENAI_API_KEY=sk-secretvalue123456 and ANTHROPIC_API_KEY: "topsecrettoken99"`
	got := String(in)
	if strings.Contains(got, "sk-secretvalue123456") || strings.Contains(got, "topsecrettoken99") {
		t.Fatalf("secret value leaked: %q", got)
	}
	// Key names are kept for readability.
	if !strings.Contains(got, "OPENAI_API_KEY") {
		t.Fatalf("key name should be preserved: %q", got)
	}
}

func TestString_EnvValue(t *testing.T) {
	t.Setenv("MY_SERVICE_TOKEN", "abc123-very-secret-value")
	got := String("the token leaked: abc123-very-secret-value end")
	if strings.Contains(got, "abc123-very-secret-value") {
		t.Fatalf("env secret leaked: %q", got)
	}
}

func TestString_PrivateKeyBlock(t *testing.T) {
	in := "-----BEGIN RSA PRIVATE KEY-----\nMIIEpAIBAAKCAQEA\n-----END RSA PRIVATE KEY-----"
	if got := String(in); strings.Contains(got, "MIIEpAIBAAKCAQEA") {
		t.Fatalf("private key leaked: %q", got)
	}
}

func TestString_NoFalsePositiveOnPlainText(t *testing.T) {
	in := "just a normal sentence about building software"
	if got := String(in); got != in {
		t.Fatalf("plain text altered: %q", got)
	}
}

func TestError(t *testing.T) {
	if Error(nil) != "" {
		t.Fatal("nil error should redact to empty string")
	}
}
