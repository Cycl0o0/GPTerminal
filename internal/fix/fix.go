package fix

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/cycl0o0/GPTerminal/internal/ai"
	"github.com/cycl0o0/GPTerminal/internal/system"
	openai "github.com/sashabaranov/go-openai"
)

func captureError(command string) string {
	cmd := exec.Command("bash", "-c", command)
	out, _ := cmd.CombinedOutput()
	s := strings.TrimSpace(string(out))
	// Limit error output to avoid huge prompts
	if len(s) > 500 {
		s = s[:500] + "..."
	}
	return s
}

func Run(ctx context.Context) error {
	lastCmd, err := system.LastCommand()
	if err != nil || lastCmd == "" {
		return fmt.Errorf("could not read last command from history: %v", err)
	}

	fmt.Printf("Last command: %s\n", lastCmd)

	client, err := ai.NewClient()
	if err != nil {
		return err
	}

	// Try to capture the error output by re-running the command
	errOutput := captureError(lastCmd)

	sysInfo := system.Detect()

	userMsg := fmt.Sprintf("Failed command: %s", lastCmd)
	if errOutput != "" {
		userMsg += fmt.Sprintf("\n\nError output:\n%s", errOutput)
	}

	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: ai.FixSystemPrompt(sysInfo.ContextBlock())},
		{Role: openai.ChatMessageRoleUser, Content: userMsg},
	}

	fmt.Print("Thinking...")
	resp, err := client.Complete(ctx, messages)
	fmt.Print("\r            \r")
	if err != nil {
		return err
	}

	suggestion := strings.TrimSpace(resp)
	if suggestion == "UNFIXABLE" {
		fmt.Println("Could not determine a fix for this command.")
		return nil
	}

	fmt.Printf("Suggested fix: \033[1;32m%s\033[0m\n", suggestion)
	fmt.Print("Execute? [Y/n] ")

	var answer string
	fmt.Scanln(&answer)
	answer = strings.TrimSpace(strings.ToLower(answer))

	if answer == "" || answer == "y" || answer == "yes" {
		return system.Execute(suggestion)
	}

	fmt.Println("Aborted.")
	return nil
}
