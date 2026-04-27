package commitmsg

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/cycl0o0/GPTerminal/internal/ai"
	"github.com/cycl0o0/GPTerminal/internal/gitutil"
	"github.com/cycl0o0/GPTerminal/internal/system"
	openai "github.com/sashabaranov/go-openai"
)

func Run(ctx context.Context, conventional, apply bool) error {
	if !gitutil.IsRepo() {
		return fmt.Errorf("not inside a git repository")
	}

	diff, err := gitutil.Diff(true)
	if err != nil {
		return err
	}
	if strings.TrimSpace(diff) == "" {
		return fmt.Errorf("no staged diff to generate a commit message from")
	}

	client, err := ai.NewClient()
	if err != nil {
		return err
	}
	sysInfo := system.Detect()
	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: ai.CommitMessageSystemPrompt(sysInfo.ContextBlock(), conventional)},
		{Role: openai.ChatMessageRoleUser, Content: fmt.Sprintf("Generate a commit message for this staged diff.\n\n```diff\n%s\n```", diff)},
	}

	fmt.Print("Generating commit message...")
	raw, err := client.Complete(ctx, messages)
	fmt.Print("\r                           \r")
	if err != nil {
		return err
	}
	message := sanitizeMessage(raw)
	fmt.Println(message)

	if !apply {
		return nil
	}

	fmt.Print("Create commit now? [Y/n] ")
	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))
	if answer != "" && answer != "y" && answer != "yes" {
		fmt.Println("Aborted.")
		return nil
	}

	cmd := exec.Command("git", "commit", "-m", message)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func sanitizeMessage(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)
	lines := strings.Split(raw, "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(strings.TrimPrefix(line, "-"))
		if line != "" {
			cleaned = append(cleaned, line)
		}
	}
	if len(cleaned) == 0 {
		return strings.TrimSpace(raw)
	}
	return strings.Join(cleaned, "\n")
}
