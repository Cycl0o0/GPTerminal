package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cycl0o0/GPTerminal/internal/ai"
	"github.com/cycl0o0/GPTerminal/internal/chatutil"
	"github.com/cycl0o0/GPTerminal/internal/session"
	"github.com/cycl0o0/GPTerminal/internal/system"
	"github.com/cycl0o0/GPTerminal/internal/tui/chat"
	openai "github.com/sashabaranov/go-openai"
	"github.com/spf13/cobra"
)

var chatSession string

var chatCmd = &cobra.Command{
	Use:   "chat [prompt...]",
	Short: "Open an AI chat in your terminal",
	Long:  "Open the interactive chat UI, or run a one-shot prompt with optional piped stdin. The assistant can inspect the current working directory with safe local tools.",
	Example: "  gpterminal chat\n" +
		"  gpterminal chat \"summarize this stack trace\"\n" +
		"  cat server.log | gpterminal chat \"what are the recurring failures?\"",
	Run: func(cmd *cobra.Command, args []string) {
		if err := runChatCommand(cmd, args, chatSession, false); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
	},
}

func init() {
	chatCmd.Flags().StringVar(&chatSession, "session", "", "Use a named chat session that can be resumed later")
	rootCmd.AddCommand(chatCmd)
}

func runChatCommand(cmd *cobra.Command, args []string, sessionName string, forceInteractive bool) error {
	client, err := ai.NewClient()
	if err != nil {
		return err
	}

	sessionName = session.NormalizeName(sessionName)
	sysInfo := system.Detect()
	piped := chatutil.HasPipedStdin(os.Stdin)
	prompt := strings.TrimSpace(strings.Join(args, " "))
	if !forceInteractive && (piped || prompt != "") {
		stdinData := ""
		if piped {
			stdinData, err = chatutil.ReadPipedStdin(os.Stdin)
			if err != nil {
				return err
			}
		}

		userMsg := chatutil.BuildUserMessage(prompt, stdinData)
		if userMsg == "" {
			return fmt.Errorf("no prompt or stdin input provided")
		}

		runner := chatutil.NewRunner(client, sysInfo)
		history, transcript := loadChatSession(sysInfo, sessionName)
		history = append(history, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: userMsg,
		})
		transcript = append(transcript, session.ChatMessage{
			Role:      openai.ChatMessageRoleUser,
			Content:   userMsg,
			Timestamp: time.Now().Format("15:04"),
		})

		reader := bufio.NewReader(os.Stdin)
		autoApprove := false
		full, finalHistory, err := runner.Stream(cmd.Context(), history, chatutil.StreamOptions{
			AllowWriteTools: true,
			OnThinking: func(text string) {
				if strings.TrimSpace(text) != "" {
					fmt.Fprintf(os.Stderr, "Thinking: %s\n", compactChatLine(text))
				}
			},
			OnToolCall: func(name, _ string) {
				fmt.Fprintf(os.Stderr, "Using tool: %s\n", name)
			},
			OnContent: func(chunk string) {
				fmt.Print(chunk)
			},
			ApproveCommand: func(req chatutil.CommandApprovalRequest) (chatutil.ApprovalDecision, error) {
				allowAuto := req.RiskErr == nil && req.Risk != nil && req.Risk.Score <= 7
				if autoApprove && allowAuto {
					fmt.Fprintf(os.Stderr, "Auto-accepted command: %s\n", req.Command)
					return chatutil.ApprovalDecision{Approved: true, AutoApprove: true}, nil
				}

				fmt.Fprintf(os.Stderr, "Command: %s\n", req.Command)
				if req.RiskErr != nil {
					fmt.Fprintf(os.Stderr, "Risk: unavailable (%v)\n", req.RiskErr)
				} else if req.Risk != nil {
					fmt.Fprintf(os.Stderr, "Risk: %d/10 [%s] %s\n", req.Risk.Score, strings.ToUpper(req.Risk.Level), req.Risk.Summary)
				}

				if allowAuto {
					fmt.Fprint(os.Stderr, "Approve? [Y]es / [a]uto / [n]o: ")
				} else {
					fmt.Fprint(os.Stderr, "Approve? [Y/n]: ")
				}
				answer, _ := reader.ReadString('\n')
				answer = strings.TrimSpace(strings.ToLower(answer))
				switch answer {
				case "a", "auto":
					if allowAuto {
						autoApprove = true
						return chatutil.ApprovalDecision{Approved: true, AutoApprove: true}, nil
					}
					return chatutil.ApprovalDecision{Approved: true}, nil
				case "n", "no", "reject":
					return chatutil.ApprovalDecision{Approved: false}, nil
				default:
					return chatutil.ApprovalDecision{Approved: true}, nil
				}
			},
			ApproveFileWrite: func(req chatutil.FileWriteApprovalRequest) (chatutil.ApprovalDecision, error) {
				fmt.Fprintf(os.Stderr, "Proposed file write: %s\n", req.Path)
				fmt.Fprintf(os.Stderr, "%s\n", req.Diff)
				fmt.Fprint(os.Stderr, "Approve file write? [Y/n]: ")
				answer, _ := reader.ReadString('\n')
				answer = strings.TrimSpace(strings.ToLower(answer))
				if answer == "n" || answer == "no" || answer == "reject" {
					return chatutil.ApprovalDecision{Approved: false}, nil
				}
				return chatutil.ApprovalDecision{Approved: true}, nil
			},
		})
		fmt.Println()
		if err != nil {
			return err
		}

		if sessionName != "" {
			transcript = append(transcript, session.ChatMessage{
				Role:      openai.ChatMessageRoleAssistant,
				Content:   full,
				Timestamp: time.Now().Format("15:04"),
			})
			if err := session.Save(&session.Record{
				Kind: session.KindChat,
				Name: sessionName,
				Chat: &session.ChatData{
					Transcript: transcript,
					History:    finalHistory,
				},
			}); err != nil {
				return err
			}
		}
		return nil
	}

	model := chat.NewModel(client, sysInfo, chat.Options{SessionName: sessionName})
	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err = p.Run()
	return err
}

func loadChatSession(sysInfo system.SystemInfo, sessionName string) ([]openai.ChatCompletionMessage, []session.ChatMessage) {
	baseHistory := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: ai.ChatSystemPrompt(sysInfo.ContextBlock())},
	}
	if sessionName == "" {
		return baseHistory, nil
	}

	record, err := session.Load(sessionName)
	if err != nil || record.Kind != session.KindChat || record.Chat == nil {
		return baseHistory, nil
	}

	history := record.Chat.History
	if len(history) == 0 {
		history = baseHistory
	}
	transcript := make([]session.ChatMessage, len(record.Chat.Transcript))
	copy(transcript, record.Chat.Transcript)
	return history, transcript
}

func compactChatLine(s string) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	if len(s) > 120 {
		return s[:120] + "..."
	}
	return s
}
