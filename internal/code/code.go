package code

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/cycl0o0/GPTerminal/internal/ai"
	"github.com/cycl0o0/GPTerminal/internal/chatutil"
	"github.com/cycl0o0/GPTerminal/internal/config"
	"github.com/cycl0o0/GPTerminal/internal/mcp"
	"github.com/cycl0o0/GPTerminal/internal/session"
	"github.com/cycl0o0/GPTerminal/internal/system"
	openai "github.com/sashabaranov/go-openai"
)

type Config struct {
	SessionName string
}

func Run(ctx context.Context, cfg Config) error {
	client, err := ai.NewClient()
	if err != nil {
		return err
	}

	sysInfo := system.Detect()
	cwd, _ := os.Getwd()

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

	projectCtx := gatherProjectContext(cwd)

	printBanner(cwd)

	messages, transcript := loadCodeSession(sysInfo, projectCtx, cfg.SessionName)

	reader := bufio.NewReader(os.Stdin)
	autoApprove := false

	for {
		fmt.Fprintf(os.Stderr, "\033[1;36m>\033[0m ")
		input, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		if strings.HasPrefix(input, "/") {
			quit := handleSlashCommand(input, &messages, &transcript, sysInfo, projectCtx, cwd, cfg.SessionName)
			if quit {
				break
			}
			continue
		}

		messages = append(messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: input,
		})
		transcript = append(transcript, session.ChatMessage{
			Role:      openai.ChatMessageRoleUser,
			Content:   input,
			Timestamp: time.Now().Format("15:04"),
		})

		text, finalHistory, err := runner.Stream(ctx, messages, chatutil.StreamOptions{
			AllowWriteTools: true,
			OnThinking: func(t string) {
				if strings.TrimSpace(t) != "" {
					line := strings.TrimSpace(strings.ReplaceAll(t, "\n", " "))
					if len(line) > 120 {
						line = line[:120] + "..."
					}
					fmt.Fprintf(os.Stderr, "\033[35m⟡ %s\033[0m\n", line)
				}
			},
			OnContent: func(chunk string) {
				fmt.Print(chunk)
			},
			OnToolCall: func(name, args string) {
				fmt.Fprintf(os.Stderr, "\033[33m⚡ %s\033[0m\n", name)
			},
			OnToolResult: func(name, result string) {
				fmt.Fprintf(os.Stderr, "\033[33m✓ %s done (%d chars)\033[0m\n", name, len(result))
			},
			ApproveCommand: func(req chatutil.CommandApprovalRequest) (chatutil.ApprovalDecision, error) {
				allowAuto := req.RiskErr == nil && req.Risk != nil && req.Risk.Score <= 7
				if autoApprove && allowAuto {
					fmt.Fprintf(os.Stderr, "\033[32m✓ auto-approved: %s\033[0m\n", req.Command)
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
				fmt.Fprintf(os.Stderr, "\n\033[1mFile:\033[0m %s\n", req.Path)
				fmt.Fprintf(os.Stderr, "%s\n", req.Diff)
				fmt.Fprint(os.Stderr, "Approve? [Y/n]: ")
				answer, _ := reader.ReadString('\n')
				answer = strings.TrimSpace(strings.ToLower(answer))
				if answer == "n" || answer == "no" {
					return chatutil.ApprovalDecision{Approved: false}, nil
				}
				return chatutil.ApprovalDecision{Approved: true}, nil
			},
		})
		fmt.Println()

		if err != nil {
			fmt.Fprintf(os.Stderr, "\033[31mError: %v\033[0m\n", err)
			continue
		}

		messages = finalHistory
		transcript = append(transcript, session.ChatMessage{
			Role:      openai.ChatMessageRoleAssistant,
			Content:   text,
			Timestamp: time.Now().Format("15:04"),
		})

		if cfg.SessionName != "" {
			saveCodeSession(cfg.SessionName, messages, transcript)
		}
	}

	if cfg.SessionName != "" {
		saveCodeSession(cfg.SessionName, messages, transcript)
	}
	return nil
}

func printBanner(cwd string) {
	fmt.Fprintf(os.Stderr, "\033[1;36m╭─────────────────────────────────────╮\033[0m\n")
	fmt.Fprintf(os.Stderr, "\033[1;36m│            GPTCode                  │\033[0m\n")
	fmt.Fprintf(os.Stderr, "\033[1;36m│   Interactive Coding Assistant      │\033[0m\n")
	fmt.Fprintf(os.Stderr, "\033[1;36m╰─────────────────────────────────────╯\033[0m\n")
	fmt.Fprintf(os.Stderr, "\033[90mProject: %s\033[0m\n", cwd)
	fmt.Fprintf(os.Stderr, "\033[90mType /help for commands, /quit to exit\033[0m\n\n")
}

func handleSlashCommand(input string, messages *[]openai.ChatCompletionMessage, transcript *[]session.ChatMessage, sysInfo system.SystemInfo, projectCtx, cwd, sessionName string) bool {
	parts := strings.Fields(input)
	cmd := strings.ToLower(parts[0])

	switch cmd {
	case "/quit", "/exit", "/q":
		fmt.Fprintf(os.Stderr, "\033[90mGoodbye!\033[0m\n")
		return true

	case "/help", "/h":
		fmt.Fprintf(os.Stderr, "\033[1mGPTCode Commands:\033[0m\n")
		fmt.Fprintf(os.Stderr, "  /help       Show this help\n")
		fmt.Fprintf(os.Stderr, "  /clear      Clear conversation and start fresh\n")
		fmt.Fprintf(os.Stderr, "  /compact    Summarize conversation to reduce context\n")
		fmt.Fprintf(os.Stderr, "  /diff       Show git diff of changes in the project\n")
		fmt.Fprintf(os.Stderr, "  /status     Show git status\n")
		fmt.Fprintf(os.Stderr, "  /undo       Undo last git change (git checkout -- .)\n")
		fmt.Fprintf(os.Stderr, "  /quit       Exit GPTCode\n")

	case "/clear":
		*messages = []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: ai.CodeSystemPrompt(sysInfo.ContextBlock(), projectCtx)},
		}
		*transcript = nil
		fmt.Fprintf(os.Stderr, "\033[90mConversation cleared.\033[0m\n")

	case "/compact":
		compactConversation(messages, sysInfo, projectCtx)

	case "/diff":
		out, err := runGitCommand(cwd, "diff")
		if err != nil {
			fmt.Fprintf(os.Stderr, "\033[31mError: %v\033[0m\n", err)
		} else if strings.TrimSpace(out) == "" {
			fmt.Fprintf(os.Stderr, "\033[90mNo changes.\033[0m\n")
		} else {
			fmt.Println(out)
		}

	case "/status":
		out, err := runGitCommand(cwd, "status", "--short")
		if err != nil {
			fmt.Fprintf(os.Stderr, "\033[31mError: %v\033[0m\n", err)
		} else if strings.TrimSpace(out) == "" {
			fmt.Fprintf(os.Stderr, "\033[90mWorking tree clean.\033[0m\n")
		} else {
			fmt.Println(out)
		}

	case "/undo":
		fmt.Fprint(os.Stderr, "\033[33mThis will discard all unstaged changes. Continue? [y/N]: \033[0m")
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer == "y" || answer == "yes" {
			out, err := runGitCommand(cwd, "checkout", "--", ".")
			if err != nil {
				fmt.Fprintf(os.Stderr, "\033[31mError: %v\033[0m\n", err)
			} else {
				fmt.Fprintf(os.Stderr, "\033[32mChanges reverted.\033[0m\n")
				if strings.TrimSpace(out) != "" {
					fmt.Println(out)
				}
			}
		} else {
			fmt.Fprintf(os.Stderr, "\033[90mCancelled.\033[0m\n")
		}

	default:
		fmt.Fprintf(os.Stderr, "\033[31mUnknown command: %s (type /help for commands)\033[0m\n", cmd)
	}

	return false
}

func compactConversation(messages *[]openai.ChatCompletionMessage, sysInfo system.SystemInfo, projectCtx string) {
	if len(*messages) <= 2 {
		fmt.Fprintf(os.Stderr, "\033[90mNothing to compact.\033[0m\n")
		return
	}

	var summary strings.Builder
	summary.WriteString("Previous conversation summary:\n")
	count := 0
	for _, msg := range *messages {
		if msg.Role == openai.ChatMessageRoleUser {
			summary.WriteString(fmt.Sprintf("- User asked: %s\n", truncate(msg.Content, 100)))
			count++
		} else if msg.Role == openai.ChatMessageRoleAssistant && msg.Content != "" {
			summary.WriteString(fmt.Sprintf("- Assistant: %s\n", truncate(msg.Content, 100)))
			count++
		}
	}

	*messages = []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: ai.CodeSystemPrompt(sysInfo.ContextBlock(), projectCtx)},
		{Role: openai.ChatMessageRoleUser, Content: summary.String()},
		{Role: openai.ChatMessageRoleAssistant, Content: "Understood. I have context from our previous conversation. How can I help?"},
	}
	fmt.Fprintf(os.Stderr, "\033[90mCompacted %d messages into summary.\033[0m\n", count)
}

func gatherProjectContext(cwd string) string {
	var ctx strings.Builder
	ctx.WriteString("Project context:\n")
	ctx.WriteString(fmt.Sprintf("Working directory: %s\n", cwd))

	if gitRoot, err := runGitCommand(cwd, "rev-parse", "--show-toplevel"); err == nil {
		ctx.WriteString(fmt.Sprintf("Git root: %s\n", strings.TrimSpace(gitRoot)))

		if branch, err := runGitCommand(cwd, "rev-parse", "--abbrev-ref", "HEAD"); err == nil {
			ctx.WriteString(fmt.Sprintf("Branch: %s\n", strings.TrimSpace(branch)))
		}

		if status, err := runGitCommand(cwd, "status", "--short"); err == nil {
			status = strings.TrimSpace(status)
			if status == "" {
				ctx.WriteString("Git status: clean\n")
			} else {
				lines := strings.Split(status, "\n")
				if len(lines) > 20 {
					lines = append(lines[:20], fmt.Sprintf("... and %d more files", len(lines)-20))
				}
				ctx.WriteString(fmt.Sprintf("Git status:\n%s\n", strings.Join(lines, "\n")))
			}
		}
	}

	if entries, err := os.ReadDir(cwd); err == nil {
		ctx.WriteString("\nTop-level files:\n")
		count := 0
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), ".") && e.Name() != ".gitignore" {
				continue
			}
			kind := "file"
			if e.IsDir() {
				kind = "dir"
			}
			ctx.WriteString(fmt.Sprintf("  [%s] %s\n", kind, e.Name()))
			count++
			if count >= 30 {
				ctx.WriteString(fmt.Sprintf("  ... and %d more\n", len(entries)-count))
				break
			}
		}
	}

	for _, name := range []string{"go.mod", "package.json", "Cargo.toml", "pyproject.toml", "requirements.txt", "pom.xml", "Makefile", "CMakeLists.txt"} {
		path := filepath.Join(cwd, name)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			data, err := os.ReadFile(path)
			if err == nil {
				content := string(data)
				if len(content) > 2000 {
					content = content[:2000] + "\n...[truncated]"
				}
				ctx.WriteString(fmt.Sprintf("\n%s:\n%s\n", name, content))
			}
			break
		}
	}

	return ctx.String()
}

func loadCodeSession(sysInfo system.SystemInfo, projectCtx, sessionName string) ([]openai.ChatCompletionMessage, []session.ChatMessage) {
	baseHistory := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: ai.CodeSystemPrompt(sysInfo.ContextBlock(), projectCtx)},
	}
	if sessionName == "" {
		return baseHistory, nil
	}

	record, err := session.Load(sessionName)
	if err != nil || record.Kind != session.KindCode || record.Chat == nil {
		return baseHistory, nil
	}

	history := record.Chat.History
	if len(history) == 0 {
		history = baseHistory
	}
	transcript := make([]session.ChatMessage, len(record.Chat.Transcript))
	copy(transcript, record.Chat.Transcript)

	fmt.Fprintf(os.Stderr, "\033[90mResumed session: %s (%d messages)\033[0m\n", sessionName, len(transcript))
	return history, transcript
}

func saveCodeSession(name string, messages []openai.ChatCompletionMessage, transcript []session.ChatMessage) {
	_ = session.Save(&session.Record{
		Kind: session.KindCode,
		Name: name,
		Chat: &session.ChatData{
			Transcript: transcript,
			History:    messages,
		},
	})
}

func runGitCommand(cwd string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = cwd
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func truncate(s string, max int) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	if len(s) > max {
		return s[:max] + "..."
	}
	return s
}
