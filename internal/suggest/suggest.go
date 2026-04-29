package suggest

import (
	"context"
	"fmt"
	"strings"

	"github.com/cycl0o0/GPTerminal/internal/ai"
	"github.com/cycl0o0/GPTerminal/internal/system"
	openai "github.com/sashabaranov/go-openai"
)

func Run(ctx context.Context, buffer string) error {
	client, err := ai.NewClient()
	if err != nil {
		return err
	}

	sysInfo := system.Detect()
	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: ai.SuggestSystemPrompt(sysInfo.ContextBlock())},
		{Role: openai.ChatMessageRoleUser, Content: buffer},
	}

	resp, err := client.Complete(ctx, messages)
	if err != nil {
		return err
	}

	command := strings.TrimSpace(resp)
	command = strings.TrimPrefix(command, "```bash")
	command = strings.TrimPrefix(command, "```sh")
	command = strings.TrimPrefix(command, "```")
	command = strings.TrimSuffix(command, "```")
	command = strings.TrimSpace(command)

	fmt.Println(command)
	return nil
}
