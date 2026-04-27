package chat

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/cycl0o0/GPTerminal/internal/ai"
	"github.com/cycl0o0/GPTerminal/internal/chatutil"
	openai "github.com/sashabaranov/go-openai"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			// Save history on exit
			_ = saveChatState(m.messages, m.history, m.sessionName)
			return m, tea.Quit

		case tea.KeyCtrlN:
			// New chat - clear everything
			sysPrompt := ai.ChatSystemPrompt(m.sysInfo.ContextBlock())
			m.messages = []chatMessage{}
			m.history = []openai.ChatCompletionMessage{
				{Role: openai.ChatMessageRoleSystem, Content: sysPrompt},
			}
			m.msgCount = 0
			m.err = nil
			m.streamBuf = ""
			m.thinkingText = ""
			m.streaming = false
			m.pendingApproval = nil
			m.notice = ""
			m.autoApproveCommands = false
			if m.ready {
				m.viewport.SetContent(m.renderMessages())
			}
			_ = saveChatState(m.messages, m.history, m.sessionName)
			return m, nil

		case tea.KeyCtrlS:
			name, err := m.saveSessionSnapshot()
			if err != nil {
				m.err = err
			} else {
				m.err = nil
				m.notice = fmt.Sprintf("Saved session: %s", name)
			}
			if m.ready {
				m.viewport.SetContent(m.renderMessages())
			}
			return m, nil

		case tea.KeyCtrlX:
			if m.streaming && m.streamCancel != nil {
				m.streamCancel()
				m.notice = "Canceling current response..."
				if m.ready {
					m.viewport.SetContent(m.renderMessages())
				}
				return m, nil
			}

		case tea.KeyEnter:
			input := strings.TrimSpace(m.textarea.Value())
			if input == "" {
				break
			}
			if m.pendingApproval != nil {
				decision, ok := parseApprovalInput(input, m.pendingApproval.allowAuto)
				if !ok {
					m.err = fmt.Errorf("approval answer must be yes%s or no", autoSuffix(m.pendingApproval.allowAuto))
					m.viewport.SetContent(m.renderMessages())
					return m, nil
				}
				m.textarea.Reset()
				if decision.AutoApprove {
					m.autoApproveCommands = true
				}
				if m.approvalCh != nil {
					m.approvalCh <- decision
				}
				m.pendingApproval = nil
				m.err = nil
				m.thinkingText = "Waiting for tool result..."
				m.notice = ""
				m.viewport.SetContent(m.renderMessages())
				m.viewport.GotoBottom()
				return m, nil
			}
			if m.streaming {
				break
			}
			if isExitInput(input) {
				_ = saveChatState(m.messages, m.history, m.sessionName)
				return m, tea.Quit
			}
			m.textarea.Reset()

			m.messages = append(m.messages, chatMessage{
				role:      "user",
				content:   input,
				timestamp: nowTimestamp(),
			})
			m.history = append(m.history, openai.ChatCompletionMessage{
				Role: openai.ChatMessageRoleUser, Content: input,
			})
			m.msgCount++

			m.streaming = true
			m.streamBuf = ""
			m.thinkingText = "Thinking..."
			m.err = nil
			m.notice = ""

			m.viewport.SetContent(m.renderMessages())
			m.viewport.GotoBottom()

			return m, m.startStream()
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		headerHeight := 3
		inputHeight := 5
		statusHeight := 1
		vpHeight := m.height - headerHeight - inputHeight - statusHeight
		if vpHeight < 1 {
			vpHeight = 1
		}

		if !m.ready {
			m.viewport = viewport.New(m.width, vpHeight)
			m.viewport.SetContent(m.renderMessages())
			m.ready = true
		} else {
			m.viewport.Width = m.width
			m.viewport.Height = vpHeight
		}

		m.textarea.SetWidth(m.width - 4)

		if m.renderer != nil {
			m.updateRenderer()
		}

		return m, nil

	case StreamChunkMsg:
		m.thinkingText = ""
		m.streamBuf += msg.Content
		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoBottom()
		return m, waitForEvent(m.eventCh)

	case ThinkingMsg:
		m.thinkingText = msg.Content
		m.notice = ""
		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoBottom()
		return m, waitForEvent(m.eventCh)

	case ToolStatusMsg:
		m.thinkingText = msg.Content
		m.notice = ""
		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoBottom()
		return m, waitForEvent(m.eventCh)

	case ApprovalRequestedMsg:
		m.pendingApproval = &approvalPrompt{
			kind:      msg.Kind,
			command:   msg.Command,
			path:      msg.Path,
			diff:      msg.Diff,
			risk:      msg.Risk,
			allowAuto: msg.AllowAuto,
		}
		if m.autoApproveCommands && msg.Kind == "command" && msg.AllowAuto && m.approvalCh != nil {
			m.pendingApproval = nil
			m.approvalCh <- chatutil.ApprovalDecision{Approved: true, AutoApprove: true}
			m.thinkingText = "Auto-approved command."
		}
		m.textarea.Reset()
		m.err = nil
		m.notice = ""
		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoBottom()
		return m, waitForEvent(m.eventCh)

	case StreamDoneMsg:
		m.streaming = false
		m.pendingApproval = nil
		m.thinkingText = ""
		m.history = msg.History
		m.messages = append(m.messages, chatMessage{
			role:      "assistant",
			content:   msg.FullContent,
			timestamp: nowTimestamp(),
		})
		m.msgCount++
		m.streamBuf = ""
		m.eventCh = nil
		m.approvalCh = nil
		m.streamCancel = nil
		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoBottom()

		// Auto-save after each response
		_ = saveChatState(m.messages, m.history, m.sessionName)
		return m, nil

	case StreamErrMsg:
		m.streaming = false
		m.err = msg.Err
		m.pendingApproval = nil
		m.thinkingText = ""
		m.streamBuf = ""
		m.eventCh = nil
		m.approvalCh = nil
		m.streamCancel = nil
		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoBottom()
		return m, nil

	case StreamCanceledMsg:
		m.streaming = false
		m.pendingApproval = nil
		m.thinkingText = ""
		m.eventCh = nil
		m.approvalCh = nil
		m.streamCancel = nil
		if strings.TrimSpace(m.streamBuf) != "" {
			m.messages = append(m.messages, chatMessage{
				role:      "assistant",
				content:   m.streamBuf,
				timestamp: nowTimestamp(),
			})
			m.history = append(m.history, openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleAssistant,
				Content: m.streamBuf,
			})
			m.msgCount++
			m.streamBuf = ""
		}
		m.notice = "Response canceled."
		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoBottom()
		_ = saveChatState(m.messages, m.history, m.sessionName)
		return m, nil
	}

	// Update textarea
	if !m.streaming || m.pendingApproval != nil {
		var taCmd tea.Cmd
		m.textarea, taCmd = m.textarea.Update(msg)
		cmds = append(cmds, taCmd)
	}

	// Update viewport
	var vpCmd tea.Cmd
	m.viewport, vpCmd = m.viewport.Update(msg)
	cmds = append(cmds, vpCmd)

	return m, tea.Batch(cmds...)
}

func isExitInput(input string) bool {
	return strings.EqualFold(strings.TrimSpace(input), "exit")
}

func (m *Model) startStream() tea.Cmd {
	history := make([]openai.ChatCompletionMessage, len(m.history))
	copy(history, m.history)
	runner := m.runner
	m.eventCh = make(chan tea.Msg, 64)
	m.approvalCh = make(chan chatutil.ApprovalDecision)
	ctx, cancel := context.WithCancel(context.Background())
	m.streamCancel = cancel

	go func(eventCh chan tea.Msg, approvalCh chan chatutil.ApprovalDecision) {
		full, finalHistory, err := runner.Stream(ctx, history, chatutil.StreamOptions{
			AllowWriteTools: true,
			OnThinking: func(text string) {
				eventCh <- ThinkingMsg{Content: text}
			},
			OnContent: func(text string) {
				eventCh <- StreamChunkMsg{Content: text}
			},
			OnToolCall: func(name, _ string) {
				eventCh <- ToolStatusMsg{Content: fmt.Sprintf("Using tool: %s", name)}
			},
			OnToolResult: func(name, _ string) {
				eventCh <- ToolStatusMsg{Content: fmt.Sprintf("Tool finished: %s", name)}
			},
			ApproveCommand: func(req chatutil.CommandApprovalRequest) (chatutil.ApprovalDecision, error) {
				eventCh <- ApprovalRequestedMsg{
					Kind:      "command",
					Command:   req.Command,
					Risk:      formatApprovalRisk(req),
					AllowAuto: req.RiskErr == nil && req.Risk != nil && req.Risk.Score <= 7,
				}
				decision := <-approvalCh
				return decision, nil
			},
			ApproveFileWrite: func(req chatutil.FileWriteApprovalRequest) (chatutil.ApprovalDecision, error) {
				eventCh <- ApprovalRequestedMsg{
					Kind: "write_file",
					Path: req.Path,
					Diff: req.Diff,
				}
				decision := <-approvalCh
				return decision, nil
			},
		})
		if errors.Is(err, context.Canceled) {
			eventCh <- StreamCanceledMsg{}
			return
		}
		if err != nil {
			eventCh <- StreamErrMsg{Err: err}
			return
		}
		eventCh <- StreamDoneMsg{FullContent: full, History: finalHistory}
	}(m.eventCh, m.approvalCh)

	return waitForEvent(m.eventCh)
}

func waitForEvent(ch chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		if ch == nil {
			return nil
		}
		return <-ch
	}
}

func formatApprovalRisk(req chatutil.CommandApprovalRequest) string {
	if req.RiskErr != nil {
		return fmt.Sprintf("Risk: unavailable (%v)", req.RiskErr)
	}
	if req.Risk == nil {
		return ""
	}
	return fmt.Sprintf("Risk: %d/10 [%s] %s", req.Risk.Score, strings.ToUpper(req.Risk.Level), req.Risk.Summary)
}

func parseApprovalInput(input string, allowAuto bool) (chatutil.ApprovalDecision, bool) {
	input = strings.TrimSpace(strings.ToLower(input))
	switch input {
	case "y", "yes":
		return chatutil.ApprovalDecision{Approved: true}, true
	case "a", "auto":
		if allowAuto {
			return chatutil.ApprovalDecision{Approved: true, AutoApprove: true}, true
		}
	case "n", "no", "reject":
		return chatutil.ApprovalDecision{Approved: false}, true
	}
	return chatutil.ApprovalDecision{}, false
}

func autoSuffix(allowAuto bool) string {
	if allowAuto {
		return ", auto,"
	}
	return ""
}

func (m *Model) saveSessionSnapshot() (string, error) {
	if m.sessionName == "" {
		m.sessionName = generatedSessionName()
	}
	if err := saveChatState(m.messages, m.history, m.sessionName); err != nil {
		return "", err
	}
	return m.sessionName, nil
}

func generatedSessionName() string {
	return "chat-" + time.Now().Format("20060102-150405")
}
