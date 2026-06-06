package gptdo

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/cycl0o0/GPTerminal/internal/ai"
	"github.com/cycl0o0/GPTerminal/internal/execution"
	"github.com/cycl0o0/GPTerminal/internal/hooks"
	"github.com/cycl0o0/GPTerminal/internal/risk"
	"github.com/cycl0o0/GPTerminal/internal/session"
	"github.com/cycl0o0/GPTerminal/internal/system"
	openai "github.com/sashabaranov/go-openai"
)

const (
	maxSteps            = 100
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

	// jsonMode makes the run non-interactive and accumulates a machine-readable
	// RunReport. humanOut receives human-facing chatter (stdout normally, stderr
	// in jsonMode so stdout stays pure JSON).
	jsonMode bool
	humanOut io.Writer
	report   *RunReport
}

// say writes human-facing output to the configured writer (stderr in JSON mode).
func (r *runner) say(format string, a ...interface{}) {
	out := r.humanOut
	if out == nil {
		out = os.Stdout
	}
	fmt.Fprintf(out, format, a...)
}

// record appends a command result to the current step's JSON report (no-op when
// not in JSON mode).
func (r *runner) record(sr *StepReport, cr CommandReport) {
	if sr != nil {
		sr.Commands = append(sr.Commands, cr)
	}
}

type commandExecution struct {
	result    system.ExecResult
	beforeDir string
	afterDir  string
}

func Run(ctx context.Context, request, sessionName string, autoApprove, jsonMode bool) error {
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
		reader:      bufio.NewReader(os.Stdin),
		autoApprove: autoApprove,
		cwd:         cwd,
		hooks:       hooks.NewRegistry(),
		jsonMode:    jsonMode,
		humanOut:    os.Stdout,
	}
	if jsonMode {
		r.humanOut = os.Stderr // keep stdout pure JSON
		r.report = &RunReport{SchemaVersion: SchemaVersion, Request: request, CWD: cwd}
	}

	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: ai.GptDoSystemPrompt(sysInfo.ContextBlock())},
		{Role: openai.ChatMessageRoleUser, Content: request},
	}

	r.say("Request: %s\n", request)
	runErr := runLoop(ctx, client, &r, request, messages, sessionName)

	if jsonMode {
		if runErr != nil && !r.report.Completed {
			r.report.Aborted = true
			msg := runErr.Error()
			r.report.Error = &msg
		}
		return emitJSON(r.report)
	}
	return runErr
}

// emitJSON writes the run report as stable JSON to stdout.
func emitJSON(rep *RunReport) error {
	b, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal gptdo report: %w", err)
	}
	fmt.Fprintln(os.Stdout, string(b))
	return nil
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
		humanOut:    os.Stdout,
	}

	r.say("Resuming session: %s\n", record.Name)
	r.say("Request: %s\n", record.GptDo.Request)
	return runLoop(ctx, client, &r, record.GptDo.Request, record.GptDo.Messages, record.Name)
}

func runLoop(ctx context.Context, client *ai.Client, r *runner, request string, messages []openai.ChatCompletionMessage, sessionName string) error {
	if err := saveSession(sessionName, request, messages, r, false, ""); err != nil {
		return err
	}

	for stepNum := 1; stepNum <= maxSteps; stepNum++ {
		r.say("Planning...")
		raw, err := client.Complete(ctx, messages)
		r.say("\r            \r")
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

		r.say("\nStep %d\n", stepNum)
		if step.Message != "" {
			r.say("%s\n", step.Message)
		}

		if step.Done {
			if step.Summary != "" {
				r.say("\n%s\n", step.Summary)
			}
			if r.report != nil {
				r.report.Completed = true
				r.report.Summary = step.Summary
			}
			if err := saveSession(sessionName, request, messages, r, true, step.Summary); err != nil {
				return err
			}
			return nil
		}

		if len(step.Commands) == 0 {
			return fmt.Errorf("AI did not return any commands")
		}

		sr := &StepReport{Index: stepNum, Message: step.Message, Proposed: step.Commands}
		report, err := r.runCommands(ctx, step.Commands, step.Rollback, sr)
		if r.report != nil {
			r.report.Steps = append(r.report.Steps, *sr)
		}
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

func (r *runner) runCommands(ctx context.Context, commands, rollbacks []string, sr *StepReport) (string, error) {
	var report strings.Builder
	rejected := false

	for idx, command := range commands {
		command = strings.TrimSpace(command)
		if command == "" {
			continue
		}

		// Authoritative, LOCAL decision (INSTRUCTIONS.md §5/§9): the LLM may
		// propose a command, but only this deterministic policy authorises it.
		// Honors the GPTERMINAL_EXEC_POLICY rollback flag for parity with the runner.
		verdict := execution.Verdict{Decision: execution.DecisionAllowed}
		if execution.PolicyEnabled() {
			verdict = execution.Classify(command)
		}
		r.say("\n[%d/%d] %s\n", idx+1, len(commands), command)
		r.say("Policy: %s%s\n", verdict.Decision, formatReasons(verdict))

		cr := CommandReport{Command: command, Decision: verdict.Decision, Reasons: verdict.Reasons}

		// Denied is refused unconditionally — autoApprove/--yes cannot override.
		// No advisory risk call is made for a command that will never run.
		if verdict.Decision == execution.DecisionDenied {
			r.say("Refused by local policy: %s\n", strings.Join(verdict.Reasons, "; "))
			rejected = true
			report.WriteString(formatDeniedCommand(command, verdict))
			r.record(sr, cr)
			break
		}

		// Advisory only — the LLM risk score is shown for context but NEVER
		// used to authorise execution. Skipped in JSON mode (non-interactive,
		// avoids a per-command network call).
		var riskResult *risk.RiskResult
		var riskErr error
		if !r.jsonMode {
			riskResult, riskErr = risk.Evaluate(ctx, command)
			if riskErr != nil {
				r.say("Risk (advisory): unavailable (%v)\n", riskErr)
			} else {
				r.say("Risk (advisory): %d/10 [%s] %s\n", riskResult.Score, strings.ToUpper(riskResult.Level), riskResult.Summary)
			}
		}
		if hint := rollbackHint(rollbacks, idx); hint != "" {
			r.say("Rollback hint: %s\n", hint)
		}

		approved, enabledAuto, err := r.approve(command, verdict)
		if err != nil {
			return "", err
		}
		if enabledAuto {
			r.autoApprove = true
		}
		if !approved {
			rejected = true
			report.WriteString(formatRejectedCommand(command, riskResult, riskErr))
			r.record(sr, cr)
			break
		}

		beforeDir := r.cwd
		result, err := r.executeCommand(command)
		if err != nil {
			return "", err
		}

		ce := commandExecution{
			result:    result,
			beforeDir: beforeDir,
			afterDir:  r.cwd,
		}

		r.printCommandResult(ce)
		report.WriteString(formatExecutedCommand(command, riskResult, ce, rollbackHint(rollbacks, idx)))

		code := result.ExitCode
		cr.Ran = true
		cr.ExitCode = &code
		cr.Output = result.Output
		r.record(sr, cr)

		if !ce.result.Success {
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

// approve decides whether a non-Denied command runs. Authorisation is driven by
// the local Verdict, never by the LLM risk score:
//   - autoApprove (--yes / auto): Allowed and NeedsConfirm run without a prompt
//     (this is exactly what --yes means; Denied was already refused upstream).
//   - interactive Allowed: prompt with a safe default of Yes; offer [a]uto.
//   - interactive NeedsConfirm: prompt fail-closed with a default of No.
func (r *runner) approve(command string, v execution.Verdict) (approved bool, enableAuto bool, err error) {
	if r.autoApprove {
		return true, false, nil
	}

	// JSON mode is non-interactive: run Allowed commands, decline NeedsConfirm
	// (those require an explicit --yes). Denied was already refused upstream.
	if r.jsonMode {
		return v.Decision == execution.DecisionAllowed, false, nil
	}

	if v.Decision == execution.DecisionNeedsConfirm {
		r.say("Higher-risk command. Execute? [y/N] ")
		answer, err := r.readAnswer()
		if err != nil {
			return false, false, err
		}
		return answer == "y" || answer == "yes", false, nil
	}

	// Allowed.
	r.say("Execute? [Y]es / [a]uto / [n]o: ")
	answer, err := r.readAnswer()
	if err != nil {
		return false, false, err
	}
	switch answer {
	case "", "y", "yes":
		return true, false, nil
	case "a", "auto":
		return true, true, nil
	default:
		return false, false, nil
	}
}

func formatReasons(v execution.Verdict) string {
	if len(v.Reasons) == 0 {
		return ""
	}
	return " — " + strings.Join(v.Reasons, "; ")
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

func (r *runner) printCommandResult(ce commandExecution) {
	r.say("Exit: %d\n", ce.result.ExitCode)
	if ce.beforeDir != ce.afterDir {
		r.say("Working directory: %s\n", ce.afterDir)
	}
	if strings.TrimSpace(ce.result.Output) == "" {
		r.say("Output: (none)\n")
		return
	}

	r.say("Output:\n")
	r.say("%s", ce.result.Output)
	if !strings.HasSuffix(ce.result.Output, "\n") {
		r.say("\n")
	}
}

// formatDeniedCommand tells the model a command was blocked by local policy so
// it proposes a safer alternative instead of retrying the same command.
func formatDeniedCommand(command string, v execution.Verdict) string {
	var b strings.Builder
	b.WriteString("Command DENIED by local security policy and not executed.\n")
	b.WriteString(fmt.Sprintf("Command: %s\n", command))
	if len(v.Reasons) > 0 {
		b.WriteString(fmt.Sprintf("Reason: %s\n", strings.Join(v.Reasons, "; ")))
	}
	b.WriteString("Do not retry this command. Propose a safer approach.\n")
	return b.String()
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
