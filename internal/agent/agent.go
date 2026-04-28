package agent

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/cycl0o0/GPTerminal/internal/ai"
	"github.com/cycl0o0/GPTerminal/internal/chatutil"
	"github.com/cycl0o0/GPTerminal/internal/config"
	"github.com/cycl0o0/GPTerminal/internal/mcp"
	"github.com/cycl0o0/GPTerminal/internal/session"
	"github.com/cycl0o0/GPTerminal/internal/system"
	openai "github.com/sashabaranov/go-openai"
)

const (
	defaultMaxSteps     = 50
	doneMarker          = "[AGENT_DONE]"
	autoAcceptMaxRisk   = 7
)

type Config struct {
	Objective   string
	SessionName string
	MaxSteps    int
	AutoApprove bool
}

func Run(ctx context.Context, cfg Config) error {
	if cfg.MaxSteps <= 0 {
		cfg.MaxSteps = defaultMaxSteps
	}

	client, err := ai.NewClient()
	if err != nil {
		return err
	}

	sysInfo := system.Detect()

	var mcpReg *mcp.Registry
	if servers := config.MCPServers(); len(servers) > 0 {
		mcpReg = mcp.NewRegistry()
		if err := mcpReg.LoadFromConfig(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: MCP: %v\n", err)
		} else {
			defer mcpReg.Close()
		}
	}

	runner := chatutil.NewRunnerWithMCP(client, sysInfo, mcpReg)

	cwd, _ := os.Getwd()
	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: ai.AgentSystemPrompt(sysInfo.ContextBlock())},
		{Role: openai.ChatMessageRoleUser, Content: cfg.Objective},
	}

	return runLoop(ctx, runner, messages, cfg, cwd, 0)
}

func Resume(ctx context.Context, sessionName string) error {
	record, err := session.Load(sessionName)
	if err != nil {
		return err
	}
	if record.Kind != session.KindAgent || record.Agent == nil {
		return fmt.Errorf("session %q is not an agent session", sessionName)
	}
	if record.Agent.Completed {
		fmt.Println("This agent session is already completed.")
		fmt.Printf("Summary: %s\n", record.Agent.Summary)
		return nil
	}

	client, err := ai.NewClient()
	if err != nil {
		return err
	}

	sysInfo := system.Detect()

	var mcpReg *mcp.Registry
	if servers := config.MCPServers(); len(servers) > 0 {
		mcpReg = mcp.NewRegistry()
		if err := mcpReg.LoadFromConfig(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: MCP: %v\n", err)
		} else {
			defer mcpReg.Close()
		}
	}

	runner := chatutil.NewRunnerWithMCP(client, sysInfo, mcpReg)

	cfg := Config{
		Objective:   record.Agent.Objective,
		SessionName: sessionName,
		MaxSteps:    defaultMaxSteps,
		AutoApprove: record.Agent.AutoApprove,
	}

	messages := record.Agent.Messages
	// Add a nudge to continue
	messages = append(messages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: "Continue where you left off. If all steps are complete, include [AGENT_DONE] and a summary.",
	})

	return runLoop(ctx, runner, messages, cfg, record.Agent.CWD, record.Agent.StepCount)
}

func runLoop(ctx context.Context, runner *chatutil.Runner, messages []openai.ChatCompletionMessage, cfg Config, cwd string, startStep int) error {
	reader := bufio.NewReader(os.Stdin)
	autoApprove := cfg.AutoApprove

	for step := startStep; step < cfg.MaxSteps; step++ {
		fmt.Fprintf(os.Stderr, "\n\033[1;34m[Agent Step %d]\033[0m\n", step+1)

		text, finalHistory, err := runner.Stream(ctx, messages, chatutil.StreamOptions{
			AllowWriteTools: true,
			OnThinking: func(t string) {
				if strings.TrimSpace(t) != "" {
					line := strings.TrimSpace(strings.ReplaceAll(t, "\n", " "))
					if len(line) > 120 {
						line = line[:120] + "..."
					}
					fmt.Fprintf(os.Stderr, "\033[35m[Thinking] %s\033[0m\n", line)
				}
			},
			OnContent: func(chunk string) {
				fmt.Print(chunk)
			},
			OnToolCall: func(name, args string) {
				fmt.Fprintf(os.Stderr, "\033[33m[Tool] %s\033[0m\n", name)
			},
			OnToolResult: func(name, result string) {
				preview := result
				if len(preview) > 200 {
					preview = preview[:200] + "..."
				}
				fmt.Fprintf(os.Stderr, "\033[33m[Tool] %s done (%d chars)\033[0m\n", name, len(result))
			},
			ApproveCommand: func(req chatutil.CommandApprovalRequest) (chatutil.ApprovalDecision, error) {
				allowAuto := req.RiskErr == nil && req.Risk != nil && req.Risk.Score <= autoAcceptMaxRisk
				if autoApprove && allowAuto {
					fmt.Fprintf(os.Stderr, "\033[32m[Auto-approved] %s\033[0m\n", req.Command)
					return chatutil.ApprovalDecision{Approved: true, AutoApprove: true}, nil
				}

				fmt.Fprintf(os.Stderr, "\n\033[1mCommand:\033[0m %s\n", req.Command)
				if req.RiskErr != nil {
					fmt.Fprintf(os.Stderr, "Risk: unavailable (%v)\n", req.RiskErr)
				} else if req.Risk != nil {
					color := "\033[32m"
					if req.Risk.Score > 3 {
						color = "\033[33m"
					}
					if req.Risk.Score > 6 {
						color = "\033[31m"
					}
					fmt.Fprintf(os.Stderr, "Risk: %s%d/10 [%s]%s %s\n", color, req.Risk.Score, strings.ToUpper(req.Risk.Level), "\033[0m", req.Risk.Summary)
				}

				prompt := "Approve? [Y]es / [a]uto / [n]o: "
				if !allowAuto {
					prompt = "Approve? [Y/n]: "
				}
				fmt.Fprint(os.Stderr, prompt)
				answer, _ := reader.ReadString('\n')
				answer = strings.TrimSpace(strings.ToLower(answer))

				switch answer {
				case "a", "auto":
					if allowAuto {
						autoApprove = true
						return chatutil.ApprovalDecision{Approved: true, AutoApprove: true}, nil
					}
					return chatutil.ApprovalDecision{Approved: true}, nil
				case "n", "no":
					return chatutil.ApprovalDecision{Approved: false}, nil
				default:
					return chatutil.ApprovalDecision{Approved: true}, nil
				}
			},
			ApproveFileWrite: func(req chatutil.FileWriteApprovalRequest) (chatutil.ApprovalDecision, error) {
				fmt.Fprintf(os.Stderr, "\n\033[1mProposed file write:\033[0m %s\n", req.Path)
				fmt.Fprintf(os.Stderr, "%s\n", req.Diff)
				fmt.Fprint(os.Stderr, "Approve file write? [Y/n]: ")
				answer, _ := reader.ReadString('\n')
				answer = strings.TrimSpace(strings.ToLower(answer))
				if answer == "n" || answer == "no" {
					return chatutil.ApprovalDecision{Approved: false}, nil
				}
				return chatutil.ApprovalDecision{Approved: true}, nil
			},
		})

		if err != nil {
			return err
		}
		fmt.Println()

		messages = finalHistory

		// Check for done marker
		if strings.Contains(text, doneMarker) {
			summary := extractSummary(text)
			fmt.Fprintf(os.Stderr, "\n\033[1;32m[Agent Complete]\033[0m\n")
			if summary != "" {
				fmt.Println(summary)
			}

			if cfg.SessionName != "" {
				saveAgentSession(cfg.SessionName, cfg.Objective, messages, cwd, autoApprove, true, summary, step+1)
			}
			return nil
		}

		// Save session progress
		if cfg.SessionName != "" {
			saveAgentSession(cfg.SessionName, cfg.Objective, messages, cwd, autoApprove, false, "", step+1)
		}

		// Add nudge for next iteration
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: "Continue. If all steps are complete, include [AGENT_DONE] and a summary.",
		})
	}

	fmt.Fprintf(os.Stderr, "\n\033[1;33m[Agent] Maximum steps (%d) reached.\033[0m\n", cfg.MaxSteps)
	if cfg.SessionName != "" {
		saveAgentSession(cfg.SessionName, cfg.Objective, messages, cwd, autoApprove, false, "", cfg.MaxSteps)
		fmt.Fprintf(os.Stderr, "Session saved. Resume with: gpterminal resume %s\n", cfg.SessionName)
	}
	return nil
}

func saveAgentSession(name, objective string, messages []openai.ChatCompletionMessage, cwd string, autoApprove, completed bool, summary string, steps int) {
	_ = session.Save(&session.Record{
		Kind: session.KindAgent,
		Name: name,
		Agent: &session.AgentData{
			Objective:   objective,
			Messages:    messages,
			CWD:         cwd,
			AutoApprove: autoApprove,
			Completed:   completed,
			Summary:     summary,
			StepCount:   steps,
		},
	})
}

func extractSummary(text string) string {
	idx := strings.Index(text, doneMarker)
	if idx < 0 {
		return ""
	}
	summary := strings.TrimSpace(text[idx+len(doneMarker):])
	return summary
}

