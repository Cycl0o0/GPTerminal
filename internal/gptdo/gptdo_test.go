package gptdo

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractJSONObject(t *testing.T) {
	raw := "```json\n{\"message\":\"ok\",\"done\":true,\"commands\":[],\"summary\":\"done\"}\n```"

	got, err := extractJSONObject(raw)
	if err != nil {
		t.Fatalf("extractJSONObject returned error: %v", err)
	}

	want := "{\"message\":\"ok\",\"done\":true,\"commands\":[],\"summary\":\"done\"}"
	if got != want {
		t.Fatalf("unexpected JSON.\nwant: %s\ngot:  %s", want, got)
	}
}

func TestParseStepTrimsCommands(t *testing.T) {
	raw := `{"message":"next","done":false,"commands":[" touch demo.sh "," chmod +x demo.sh "],"summary":""}`

	step, err := parseStep(raw)
	if err != nil {
		t.Fatalf("parseStep returned error: %v", err)
	}

	if len(step.Commands) != 2 {
		t.Fatalf("expected 2 commands, got %d", len(step.Commands))
	}
	if step.Commands[0] != "touch demo.sh" {
		t.Fatalf("unexpected first command: %q", step.Commands[0])
	}
	if step.Commands[1] != "chmod +x demo.sh" {
		t.Fatalf("unexpected second command: %q", step.Commands[1])
	}
}

func TestExtractCWDMarker(t *testing.T) {
	output := "line 1\n" + cwdMarker + "/tmp/demo\n"

	cwd, cleaned := extractCWDMarker(output)
	if cwd != "/tmp/demo" {
		t.Fatalf("unexpected cwd: %q", cwd)
	}
	if cleaned != "line 1\n" {
		t.Fatalf("unexpected cleaned output: %q", cleaned)
	}
}

func TestExecuteCommandTracksDirectoryChanges(t *testing.T) {
	baseDir := t.TempDir()
	childDir := filepath.Join(baseDir, "child")
	if err := os.Mkdir(childDir, 0755); err != nil {
		t.Fatalf("mkdir child: %v", err)
	}

	r := runner{
		reader: bufio.NewReader(strings.NewReader("")),
		cwd:    baseDir,
	}

	result, err := r.executeCommand("cd child")
	if err != nil {
		t.Fatalf("executeCommand returned error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, got %+v", result)
	}
	if r.cwd != childDir {
		t.Fatalf("expected cwd %q, got %q", childDir, r.cwd)
	}

	pwdResult, err := r.executeCommand("pwd")
	if err != nil {
		t.Fatalf("executeCommand pwd returned error: %v", err)
	}
	if strings.TrimSpace(pwdResult.Output) != childDir {
		t.Fatalf("expected pwd output %q, got %q", childDir, strings.TrimSpace(pwdResult.Output))
	}
}
