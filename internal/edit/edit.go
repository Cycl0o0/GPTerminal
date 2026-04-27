package edit

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cycl0o0/GPTerminal/internal/ai"
	"github.com/cycl0o0/GPTerminal/internal/diffutil"
	"github.com/cycl0o0/GPTerminal/internal/fileutil"
	"github.com/cycl0o0/GPTerminal/internal/system"
	openai "github.com/sashabaranov/go-openai"
)

type response struct {
	Summary string `json:"summary"`
	Content string `json:"content"`
}

func Run(ctx context.Context, path, instruction string) error {
	client, err := ai.NewClient()
	if err != nil {
		return err
	}

	path = filepath.Clean(strings.TrimSpace(path))
	if path == "" {
		return fmt.Errorf("file path cannot be empty")
	}

	current := ""
	if _, err := os.Stat(path); err == nil {
		current, err = fileutil.ReadText(path)
		if err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	sysInfo := system.Detect()
	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: ai.EditSystemPrompt(sysInfo.ContextBlock())},
		{Role: openai.ChatMessageRoleUser, Content: fmt.Sprintf("Target file: %s\n\nInstruction:\n%s\n\nCurrent content:\n```text\n%s\n```", path, instruction, current)},
	}

	fmt.Print("Planning edit...")
	raw, err := client.Complete(ctx, messages)
	fmt.Print("\r               \r")
	if err != nil {
		return err
	}

	plan, err := parseResponse(raw)
	if err != nil {
		return err
	}

	diff := diffutil.Unified(path, current, plan.Content)
	if plan.Summary != "" {
		fmt.Println(plan.Summary)
	}
	fmt.Printf("Diff for %s:\n%s\n", path, diff)
	if strings.TrimSpace(diff) == "No changes." {
		return nil
	}

	fmt.Print("Apply changes? [Y/n] ")
	var answer string
	fmt.Scanln(&answer)
	answer = strings.TrimSpace(strings.ToLower(answer))
	if answer != "" && answer != "y" && answer != "yes" {
		fmt.Println("Aborted.")
		return nil
	}

	return fileutil.WriteText(path, plan.Content)
}

func parseResponse(raw string) (*response, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start == -1 || end == -1 || end < start {
		return nil, fmt.Errorf("no JSON object found in edit response")
	}

	var resp response
	if err := json.Unmarshal([]byte(raw[start:end+1]), &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
