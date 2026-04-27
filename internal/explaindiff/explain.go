package explaindiff

import (
	"context"
	"fmt"
	"strings"

	"github.com/cycl0o0/GPTerminal/internal/ai"
	"github.com/cycl0o0/GPTerminal/internal/gitutil"
	"github.com/cycl0o0/GPTerminal/internal/system"
	openai "github.com/sashabaranov/go-openai"
)

func Run(ctx context.Context, staged bool, paths []string) (string, error) {
	if !gitutil.IsRepo() {
		return "", fmt.Errorf("not inside a git repository")
	}

	diff, err := gitutil.Diff(staged, paths...)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(diff) == "" {
		if staged {
			return "", fmt.Errorf("no staged diff to explain")
		}
		return "", fmt.Errorf("no diff to explain")
	}

	client, err := ai.NewClient()
	if err != nil {
		return "", err
	}
	sysInfo := system.Detect()
	scope := "working tree"
	if staged {
		scope = "staged"
	}
	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: ai.ExplainDiffSystemPrompt(sysInfo.ContextBlock())},
		{Role: openai.ChatMessageRoleUser, Content: fmt.Sprintf("Explain this %s git diff.\n\n```diff\n%s\n```", scope, diff)},
	}
	return client.Complete(ctx, messages)
}
