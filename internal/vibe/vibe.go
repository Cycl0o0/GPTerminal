package vibe

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/cycl0o0/GPTerminal/internal/ai"
	"github.com/cycl0o0/GPTerminal/internal/system"
	openai "github.com/sashabaranov/go-openai"
)

func Run(ctx context.Context, description string, autoYes bool) error {
	client, err := ai.NewClient()
	if err != nil {
		return err
	}

	sysInfo := system.Detect()
	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: ai.VibeSystemPrompt(sysInfo.ContextBlock())},
		{Role: openai.ChatMessageRoleUser, Content: description},
	}

	fmt.Fprint(os.Stderr, "Generating command...")
	resp, err := client.Complete(ctx, messages)
	fmt.Fprint(os.Stderr, "\r                     \r")
	if err != nil {
		return err
	}

	command := strings.TrimSpace(resp)
	// Strip code fences if present
	command = strings.TrimPrefix(command, "```bash")
	command = strings.TrimPrefix(command, "```sh")
	command = strings.TrimPrefix(command, "```")
	command = strings.TrimSuffix(command, "```")
	command = strings.TrimSpace(command)

	// Auto-yes mode: print and execute without prompting
	if autoYes {
		fmt.Println(command)
		return system.Execute(command)
	}

	// Non-TTY stdout: just print the raw command
	if !isStdoutTTY() {
		fmt.Println(command)
		return nil
	}

	fmt.Printf("Command: \033[1;36m%s\033[0m\n", command)
	fmt.Print("[Y]es / [n]o / [e]dit: ")

	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))

	switch answer {
	case "", "y", "yes":
		return system.Execute(command)
	case "e", "edit":
		fmt.Printf("Edit command: ")
		edited, _ := reader.ReadString('\n')
		edited = strings.TrimSpace(edited)
		if edited != "" {
			return system.Execute(edited)
		}
		fmt.Println("Empty command, aborted.")
		return nil
	default:
		fmt.Println("Aborted.")
		return nil
	}
}

func isStdoutTTY() bool {
	info, err := os.Stdout.Stat()
	if err != nil {
		return true
	}
	return info.Mode()&os.ModeCharDevice != 0
}
