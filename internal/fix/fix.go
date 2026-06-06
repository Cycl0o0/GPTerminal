package fix

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cycl0o0/GPTerminal/internal/ai"
	"github.com/cycl0o0/GPTerminal/internal/execution"
	"github.com/cycl0o0/GPTerminal/internal/system"
	openai "github.com/sashabaranov/go-openai"
)

// captureError re-runs the failed command to collect its error output. Rerunning
// is itself execution, so it is policy-gated: only an `allowed` command is rerun.
// A needs_confirm/denied command (it may be destructive) is never rerun just to
// gather text — the fix can still be suggested without it.
func captureError(ctx context.Context, command string) string {
	if execution.PolicyEnabled() && execution.Classify(command).Decision != execution.DecisionAllowed {
		return ""
	}
	r := execution.NewRunner()
	r.AssumeYes = true
	r.RedactOutput = true
	res, err := r.Run(ctx, execution.Command{Raw: command})
	if err != nil || res == nil {
		return ""
	}
	s := strings.TrimSpace(res.Stdout + res.Stderr)
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

	// Try to capture the error output by re-running the command (policy-gated).
	errOutput := captureError(ctx, lastCmd)

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
		// system.Execute routes through the central policy; a denied suggestion
		// is refused even though the user said yes.
		if err := system.Execute(suggestion); err != nil {
			if errors.Is(err, execution.ErrCommandDenied) {
				fmt.Println("Suggested fix was blocked by local policy and not executed.")
				return nil
			}
			return err
		}
		return nil
	}

	fmt.Println("Aborted.")
	return nil
}
