package run

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/cycl0o0/GPTerminal/internal/ai"
	"github.com/cycl0o0/GPTerminal/internal/risk"
	"github.com/cycl0o0/GPTerminal/internal/system"
	openai "github.com/sashabaranov/go-openai"
)

type response struct {
	Message string `json:"message"`
	Command string `json:"command"`
}

func Run(ctx context.Context, request string) error {
	client, err := ai.NewClient()
	if err != nil {
		return err
	}
	sysInfo := system.Detect()
	reader := bufio.NewReader(os.Stdin)

	plan, err := generate(ctx, client, sysInfo, request)
	if err != nil {
		return err
	}
	command := strings.TrimSpace(plan.Command)
	if command == "" {
		return fmt.Errorf("AI did not return a command")
	}

	for {
		if plan.Message != "" {
			fmt.Println(plan.Message)
		}
		fmt.Printf("Command: %s\n", command)
		if rr, err := risk.Evaluate(ctx, command); err == nil {
			fmt.Printf("Risk: %d/10 [%s] %s\n", rr.Score, strings.ToUpper(rr.Level), rr.Summary)
		}
		fmt.Print("Execute? [Y]es / [e]dit / [n]o: ")
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		switch answer {
		case "e", "edit":
			fmt.Print("Edit command: ")
			edited, _ := reader.ReadString('\n')
			command = strings.TrimSpace(edited)
			continue
		case "n", "no":
			fmt.Println("Aborted.")
			return nil
		}
		break
	}

	result, err := system.ExecuteCapture(command)
	if err != nil {
		return err
	}
	if strings.TrimSpace(result.Output) != "" {
		fmt.Print(result.Output)
	}
	if result.Success {
		return nil
	}

	fmt.Printf("Exit code: %d\n", result.ExitCode)
	fmt.Print("Ask AI for a retry command? [Y/n] ")
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))
	if answer != "" && answer != "y" && answer != "yes" {
		return nil
	}

	retry, err := generateRetry(ctx, client, sysInfo, request, command, result.Output)
	if err != nil {
		return err
	}
	retryCommand := strings.TrimSpace(retry.Command)
	if retryCommand == "" {
		return fmt.Errorf("AI did not return a retry command")
	}

	fmt.Printf("Retry command: %s\n", retryCommand)
	fmt.Print("Execute retry? [Y/n] ")
	answer, _ = reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))
	if answer != "" && answer != "y" && answer != "yes" {
		return nil
	}

	return system.Execute(retryCommand)
}

func generate(ctx context.Context, client *ai.Client, sysInfo system.SystemInfo, request string) (*response, error) {
	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: ai.RunSystemPrompt(sysInfo.ContextBlock())},
		{Role: openai.ChatMessageRoleUser, Content: request},
	}
	raw, err := client.Complete(ctx, messages)
	if err != nil {
		return nil, err
	}
	return parseResponse(raw)
}

func generateRetry(ctx context.Context, client *ai.Client, sysInfo system.SystemInfo, request, command, output string) (*response, error) {
	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: ai.RunRetrySystemPrompt(sysInfo.ContextBlock())},
		{Role: openai.ChatMessageRoleUser, Content: fmt.Sprintf("Original request: %s\n\nCommand: %s\n\nOutput:\n%s", request, command, output)},
	}
	raw, err := client.Complete(ctx, messages)
	if err != nil {
		return nil, err
	}
	return parseResponse(raw)
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
		return nil, fmt.Errorf("no JSON object found in run response")
	}
	var resp response
	if err := json.Unmarshal([]byte(raw[start:end+1]), &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
