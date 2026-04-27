package review

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/cycl0o0/GPTerminal/internal/ai"
	"github.com/cycl0o0/GPTerminal/internal/fileutil"
	"github.com/cycl0o0/GPTerminal/internal/gitutil"
	"github.com/cycl0o0/GPTerminal/internal/system"
	openai "github.com/sashabaranov/go-openai"
)

const maxReviewChars = 120000

func Run(ctx context.Context, path string, staged bool) (string, error) {
	client, err := ai.NewClient()
	if err != nil {
		return "", err
	}

	input, err := buildInput(path, staged)
	if err != nil {
		return "", err
	}

	sysInfo := system.Detect()
	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: ai.ReviewSystemPrompt(sysInfo.ContextBlock())},
		{Role: openai.ChatMessageRoleUser, Content: input},
	}
	return client.Complete(ctx, messages)
}

func buildInput(path string, staged bool) (string, error) {
	if path != "" {
		content, err := fileutil.ReadText(path)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Review this file for bugs, risks, regressions, and missing tests.\n\nFile: %s\n\n```text\n%s\n```", filepath.Clean(path), truncate(content, maxReviewChars)), nil
	}

	if !gitutil.IsRepo() {
		return "", fmt.Errorf("not inside a git repository")
	}

	diff, err := gitutil.Diff(staged)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(diff) == "" {
		if staged {
			return "", fmt.Errorf("no staged diff to review")
		}
		return "", fmt.Errorf("no working tree diff to review")
	}

	scope := "working tree diff"
	if staged {
		scope = "staged diff"
	}
	return fmt.Sprintf("Review this %s for bugs, risks, regressions, and missing tests.\n\n```diff\n%s\n```", scope, truncate(diff, maxReviewChars)), nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "\n...[truncated]"
}
