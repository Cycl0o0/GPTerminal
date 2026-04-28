package risk

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cycl0o0/GPTerminal/internal/ai"
	"github.com/cycl0o0/GPTerminal/internal/system"
	openai "github.com/sashabaranov/go-openai"
)

type RiskResult struct {
	Score   int      `json:"score"`
	Level   string   `json:"level"`
	Summary string   `json:"summary"`
	Risks   []string `json:"risks"`
}

func Evaluate(ctx context.Context, command string) (*RiskResult, error) {
	client, err := ai.NewClient()
	if err != nil {
		return nil, err
	}

	sysInfo := system.Detect()
	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: ai.RiskSystemPrompt(sysInfo.ContextBlock())},
		{Role: openai.ChatMessageRoleUser, Content: command},
	}

	resp, err := client.Complete(ctx, messages)
	if err != nil {
		return nil, err
	}

	// Extract the JSON object from the response, ignoring any surrounding text
	resp = strings.TrimSpace(resp)
	resp = strings.TrimPrefix(resp, "```json")
	resp = strings.TrimPrefix(resp, "```")
	resp = strings.TrimSuffix(resp, "```")
	resp = strings.TrimSpace(resp)

	// Find the first '{' and last '}' to extract just the JSON object
	start := strings.Index(resp, "{")
	end := strings.LastIndex(resp, "}")
	if start == -1 || end == -1 || end <= start {
		return nil, fmt.Errorf("no valid JSON found in response:\n%s", resp)
	}
	jsonStr := resp[start : end+1]

	var result RiskResult
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("failed to parse risk response: %w\nRaw: %s", err, jsonStr)
	}
	return &result, nil
}

func PrintResult(r *RiskResult) {
	var color string
	switch {
	case r.Score <= 3:
		color = "\033[1;32m" // green
	case r.Score <= 6:
		color = "\033[1;33m" // yellow
	default:
		color = "\033[1;31m" // red
	}
	reset := "\033[0m"

	fmt.Printf("\n%sRisk Score: %d/10 [%s]%s\n", color, r.Score, strings.ToUpper(r.Level), reset)
	fmt.Printf("%s%s%s\n", color, r.Summary, reset)

	if len(r.Risks) > 0 {
		fmt.Println("\nRisks:")
		for _, risk := range r.Risks {
			fmt.Printf("  %s• %s%s\n", color, risk, reset)
		}
	}
	fmt.Println()
}

func PrintResultPlain(r *RiskResult) {
	fmt.Printf("\nRisk Score: %d/10 [%s]\n", r.Score, strings.ToUpper(r.Level))
	fmt.Println(r.Summary)

	if len(r.Risks) > 0 {
		fmt.Println("\nRisks:")
		for _, risk := range r.Risks {
			fmt.Printf("  - %s\n", risk)
		}
	}
	fmt.Println()
}
