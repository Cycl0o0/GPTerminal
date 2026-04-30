package gptdo

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/cycl0o0/GPTerminal/internal/ai"
	"github.com/cycl0o0/GPTerminal/internal/hooks"
	"github.com/cycl0o0/GPTerminal/internal/risk"
	"github.com/cycl0o0/GPTerminal/internal/session"
	"github.com/cycl0o0/GPTerminal/internal/system"
	openai "github.com/sashabaranov/go-openai"
)

const (
	maxSteps            = 100
	autoAcceptMaxRisk   = 7
	maxCommandOutputLen = 4000
	cwdMarker           = "__GPTDO_CWD__:"
)

type stepResponse struct {
	Message  string   `json:"message"`
	Done     bool     `json:"done"`
	Commands []string `json:"commands"`
	Rollback []string `json:"rollback"`
	Summary  string   `json:"summary"`
}

type runner struct {
	reader      *bufio.Reader
	autoApprove bool
	cwd         string
	hooks       *hooks.Registry
}

type commandExecution struct {
	result    system.ExecResult
	beforeDir string
	afterDir  string
}

func Run(ctx context.Context, request, sessionName string) error {
	client, err := ai.NewClient()
	if err != nil {
		return err
	}

	sysInfo := system.Detect()
	cwd, err := os.Getwd()
	if err != nil {
		cwd = sysInfo.WorkDir
	}

	r := runner{
		reader: bufio.NewReader(os.Stdin),
		cwd:    cwd,
		hooks:  hooks.NewRegistry(),
	}

	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: ai.GptDoSystemPrompt(sysInfo.ContextBlock())},
		{Role: openai.ChatMessageRoleUser, Content: request},
	}

	fmt.Printf("Request: %s\n", request)
	return runLoop(ctx, client, &r, request, messages, sessionName)
}

func Resume(ctx context.Context, sessionName string) error {
	record, err := session.Load(sessionName)
	if err != nil {
		return err
	}
	if record.Kind != session.KindGptDo || record.GptDo == nil {
		return fmt.Errorf("session %q is not a gptdo session", sessionName)
	}
	if record.GptDo.Completed {
		if record.GptDo.Summary != "" {
			fmt.Println(record.GptDo.Summary)
		}
		return nil
	}

	client, err := ai.NewClient()
	if err != nil {
		return err
	}
	r := runner{
		reader:      bufio.NewReader(os.Stdin),
		autoApprove: record.GptDo.AutoApprove,
		cwd:         record.GptDo.CWD,
		hooks:       hooks.NewRegistry(),
	}

	fmt.Printf("Resuming session: %s\n", record.Name)
	fmt.Printf("Request: %s\n", record.GptDo.Request)
	return runLoop(ctx, client, &r, record.GptDo.Request, record.GptDo.Messages, record.Name)
}

func runLoop(ctx context.Context, client *ai.Client, r *runner, request string, messages []openai.ChatCompletionMessage, sessionName string) error {
	if err := saveSession(sessionName, request, messages, r, false, ""); err != nil {
		return err
	}

	for stepNum := 1; stepNum <= maxSteps; stepNum++ {
		fmt.Print("Planning...")
		raw, err := client.Complete(ctx, messages)
		fmt.Print("\r            \r")
		if err != nil {
			return err
		}

		step, err := parseStep(raw)
		if err != nil {
			return fmt.Errorf("parse gptdo response: %w", err)
		}

		messages = append(messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleAssistant,
			Content: strings.TrimSpace(raw),
		})

		fmt.Printf("\nStep %d\n", stepNum)
		if step.Message != "" {
			fmt.Printf("%s\n", step.Message)
		}

		if step.Done {
			if step.Summary != "" {
				fmt.Printf("\n%s\n", step.Summary)
			}
			if err := saveSession(sessionName, request, messages, r, true, step.Summary); err != nil {
				return err
			}
			return nil
		}

		if len(step.Commands) == 0 {
			return fmt.Errorf("AI did not return any commands")
		}

		report, err := r.runCommands(ctx, step.Commands, step.Rollback)
		if err != nil {
			return err
		}

		messages = append(messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: report,
		})
		if err := saveSession(sessionName, request, messages, r, false, ""); err != nil {
			return err
		}
	}

	return fmt.Errorf("stopped after %d steps without completion", maxSteps)
}

func (r *runner) runCommands(ctx context.Context, commands, rollbacks []string) (string, error) {
	var report strings.Builder
	rejected := false

	for idx, command := range commands {
		command = strings.TrimSpace(command)
		if command == "" {
			continue
		}

		riskResult, riskErr := risk.Evaluate(ctx, command)
		fmt.Printf("\n[%d/%d] %s\n", idx+1, len(commands), command)
		if riskErr != nil {
			fmt.Printf("Risk: unavailable (%v)\n", riskErr)
		} else {
			fmt.Printf("Risk: %d/10 [%s] %s\n", riskResult.Score, strings.ToUpper(riskResult.Level), riskResult.Summary)
		}
		if hint := rollbackHint(rollbacks, idx); hint != "" {
			fmt.Printf("Rollback hint: %s\n", hint)
		}

		approved, enabledAuto, err := r.approve(command, riskResult, riskErr)
		if err != nil {
			return "", err
		}
		if enabledAuto {
			r.autoApprove = true
		}
		if !approved {
			rejected = true
			report.WriteString(formatRejectedCommand(command, riskResult, riskErr))
			break
		}

		beforeDir := r.cwd
		result, err := r.executeCommand(command)
		if err != nil {
			return "", err
		}

		execution := commandExecution{
			result:    result,
			beforeDir: beforeDir,
			afterDir:  r.cwd,
		}

		printCommandResult(execution)
		report.WriteString(formatExecutedCommand(command, riskResult, execution, rollbackHint(rollbacks, idx)))

		if !execution.result.Success {
			break
		}
	}

	if rejected {
		report.WriteString("\nThe user rejected the command. Propose a different approach.\n")
		return report.String(), nil
	}

	report.WriteString("\nContinue only if more commands are still needed.\n")
	return report.String(), nil
}

func (r *runner) approve(command string, rr *risk.RiskResult, riskErr error) (approved bool, enableAuto bool, err error) {
	allowAuto := riskErr == nil && rr != nil && rr.Score <= autoAcceptMaxRisk
	requiresManual := !r.autoApprove || riskErr != nil || (rr != nil && rr.Score > autoAcceptMaxRisk)
	if !requiresManual {
		fmt.Printf("Auto-accepted (risk %d/10 <= %d/10).\n", rr.Score, autoAcceptMaxRisk)
		return true, false, nil
	}

	if r.autoApprove {
		switch {
		case riskErr != nil:
			fmt.Println("Auto-accept bypassed because risk evaluation failed.")
		case rr != nil:
			fmt.Printf("Auto-accept bypassed because risk is %d/10 > %d/10.\n", rr.Score, autoAcceptMaxRisk)
		}
		fmt.Print("Execute? [Y/n] ")
		answer, err := r.readAnswer()
		if err != nil {
			return false, false, err
		}
		return answer == "" || answer == "y" || answer == "yes", false, nil
	}

	if !allowAuto {
		switch {
		case riskErr != nil:
			fmt.Println("Auto-accept is unavailable because risk evaluation failed.")
		case rr != nil:
			fmt.Printf("Auto-accept is unavailable because risk is %d/10 > %d/10.\n", rr.Score, autoAcceptMaxRisk)
		}
		fmt.Print("Execute? [Y/n] ")
		answer, err := r.readAnswer()
		if err != nil {
			return false, false, err
		}
		return answer == "" || answer == "y" || answer == "yes", false, nil
	}

	fmt.Print("Execute? [Y]es / [a]uto / [n]o: ")
	answer, err := r.readAnswer()
	if err != nil {
		return false, false, err
	}

	switch answer {
	case "", "y", "yes":
		return true, false, nil
	case "a", "auto":
		if riskErr == nil && rr != nil && rr.Score <= autoAcceptMaxRisk {
			return true, true, nil
		}
		fmt.Println("Auto-accept is only enabled for commands at or below the 7/10 risk threshold.")
		return true, false, nil
	default:
		return false, false, nil
	}
}

func (r *runner) readAnswer() (string, error) {
	answer, err := r.reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(strings.ToLower(answer)), nil
}

func parseStep(raw string) (*stepResponse, error) {
	jsonStr, err := extractJSONObject(raw)
	if err != nil {
		return nil, err
	}

	var step stepResponse
	if err := json.Unmarshal([]byte(jsonStr), &step); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	for i, command := range step.Commands {
		step.Commands[i] = strings.TrimSpace(command)
	}
	for i, rollback := range step.Rollback {
		step.Rollback[i] = strings.TrimSpace(rollback)
	}

	return &step, nil
}

func extractJSONObject(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	trimmed = strings.TrimPrefix(trimmed, "```json")
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimSuffix(trimmed, "```")
	trimmed = strings.TrimSpace(trimmed)

	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start == -1 || end == -1 || end < start {
		return "", fmt.Errorf("no JSON object found in response:\n%s", raw)
	}
	return trimmed[start : end+1], nil
}

func (r *runner) executeCommand(command string) (system.ExecResult, error) {
	if r.hooks != nil {
		r.hooks.Fire(context.Background(), hooks.PreCommand, &hooks.CommandContext{
			Command: command,
			WorkDir: r.cwd,
		})
	}

	var result system.ExecResult
	var err error
	if isPureCDCommand(command) {
		result, err = r.executeCD(command)
	} else {
		result, err = system.ExecuteCaptureInDir(command, r.cwd)
	}

	if r.hooks != nil {
		r.hooks.Fire(context.Background(), hooks.PostCommand, &hooks.CommandResult{
			Command:  command,
			ExitCode: result.ExitCode,
			Output:   result.Output,
			Err:      err,
		})
	}

	return result, err
}

func (r *runner) executeCD(command string) (system.ExecResult, error) {
	script := command + "\nstatus=$?\nif [ $status -ne 0 ]; then exit $status; fi\nprintf '" + cwdMarker + "%s\\n' \"$PWD\"\n"

	result, err := system.ExecuteCaptureInDir(script, r.cwd)
	if err != nil {
		return result, err
	}

	newDir, cleanedOutput := extractCWDMarker(result.Output)
	result.Output = cleanedOutput
	if result.Success && newDir != "" {
		r.cwd = newDir
	}

	return result, nil
}

func isPureCDCommand(command string) bool {
	trimmed := strings.TrimSpace(command)
	if trimmed == "cd" {
		return true
	}
	if !strings.HasPrefix(trimmed, "cd ") {
		return false
	}
	return !strings.ContainsAny(trimmed, "&;|<>()\n")
}

func extractCWDMarker(output string) (string, string) {
	lines := strings.Split(strings.ReplaceAll(output, "\r\n", "\n"), "\n")
	filtered := make([]string, 0, len(lines))
	var cwd string

	for _, line := range lines {
		if strings.HasPrefix(line, cwdMarker) {
			cwd = strings.TrimSpace(strings.TrimPrefix(line, cwdMarker))
			continue
		}
		filtered = append(filtered, line)
	}

	cleaned := strings.Join(filtered, "\n")
	cleaned = strings.TrimSuffix(cleaned, "\n")
	if output != "" && strings.HasSuffix(output, "\n") && cleaned != "" {
		cleaned += "\n"
	}

	return cwd, cleaned
}

func printCommandResult(execution commandExecution) {
	fmt.Printf("Exit: %d\n", execution.result.ExitCode)
	if execution.beforeDir != execution.afterDir {
		fmt.Printf("Working directory: %s\n", execution.afterDir)
	}
	if strings.TrimSpace(execution.result.Output) == "" {
		fmt.Println("Output: (none)")
		return
	}

	fmt.Println("Output:")
	fmt.Print(execution.result.Output)
	if !strings.HasSuffix(execution.result.Output, "\n") {
		fmt.Println()
	}
}

func formatRejectedCommand(command string, rr *risk.RiskResult, riskErr error) string {
	var b strings.Builder
	b.WriteString("Command rejected by user.\n")
	b.WriteString(fmt.Sprintf("Command: %s\n", command))
	if riskErr != nil {
		b.WriteString(fmt.Sprintf("Risk: unavailable (%v)\n", riskErr))
	} else if rr != nil {
		b.WriteString(fmt.Sprintf("Risk: %d/10 [%s] %s\n", rr.Score, rr.Level, rr.Summary))
	}
	return b.String()
}

func formatExecutedCommand(command string, rr *risk.RiskResult, execution commandExecution, rollback string) string {
	var b strings.Builder
	b.WriteString("Command executed.\n")
	b.WriteString(fmt.Sprintf("Command: %s\n", command))
	if rr != nil {
		b.WriteString(fmt.Sprintf("Risk: %d/10 [%s] %s\n", rr.Score, rr.Level, rr.Summary))
	}
	if rollback != "" {
		b.WriteString(fmt.Sprintf("Rollback hint: %s\n", rollback))
	}
	b.WriteString(fmt.Sprintf("Working directory before: %s\n", execution.beforeDir))
	b.WriteString(fmt.Sprintf("Working directory after: %s\n", execution.afterDir))
	b.WriteString(fmt.Sprintf("Exit code: %d\n", execution.result.ExitCode))
	if strings.TrimSpace(execution.result.Output) == "" {
		b.WriteString("Output: (none)\n")
	} else {
		output := execution.result.Output
		if len(output) > maxCommandOutputLen {
			output = output[:maxCommandOutputLen] + "\n...[truncated]"
		}
		b.WriteString("Output:\n")
		b.WriteString(output)
		if !strings.HasSuffix(output, "\n") {
			b.WriteString("\n")
		}
	}
	if execution.result.Success {
		b.WriteString("Status: success\n")
	} else {
		b.WriteString("Status: failure\n")
	}
	b.WriteString("\n")
	return b.String()
}

func rollbackHint(rollbacks []string, idx int) string {
	if idx < 0 || idx >= len(rollbacks) {
		return ""
	}
	return strings.TrimSpace(rollbacks[idx])
}

func saveSession(sessionName, request string, messages []openai.ChatCompletionMessage, r *runner, completed bool, summary string) error {
	if strings.TrimSpace(sessionName) == "" {
		return nil
	}
	return session.Save(&session.Record{
		Kind: session.KindGptDo,
		Name: sessionName,
		GptDo: &session.GptDoData{
			Request:     request,
			Messages:    messages,
			CWD:         r.cwd,
			AutoApprove: r.autoApprove,
			Completed:   completed,
			Summary:     summary,
		},
	})
}
