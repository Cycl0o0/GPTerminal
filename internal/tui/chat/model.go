package chat

import (
	"context"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/cycl0o0/GPTerminal/internal/ai"
	"github.com/cycl0o0/GPTerminal/internal/chatutil"
	"github.com/cycl0o0/GPTerminal/internal/system"
	openai "github.com/sashabaranov/go-openai"
)

type Options struct {
	SessionName string
}

type chatMessage struct {
	role      string
	content   string
	thinking  string
	timestamp string
}

type approvalPrompt struct {
	kind      string
	command   string
	path      string
	diff      string
	risk      string
	allowAuto bool
}

type Model struct {
	viewport            viewport.Model
	textarea            textarea.Model
	messages            []chatMessage
	history             []openai.ChatCompletionMessage
	runner              *chatutil.Runner
	sysInfo             system.SystemInfo
	sessionName         string
	streaming           bool
	streamBuf           string
	thinkingText        string
	reasoningBuf        string
	err                 error
	width               int
	height              int
	ready               bool
	renderer            *glamour.TermRenderer
	msgCount            int
	eventCh             chan tea.Msg
	approvalCh          chan chatutil.ApprovalDecision
	pendingApproval     *approvalPrompt
	autoApproveCommands bool
	streamCancel        context.CancelFunc
	notice              string
}

func NewModel(client *ai.Client, sysInfo system.SystemInfo, opts Options) Model {
	ta := textarea.New()
	ta.Placeholder = "Ask anything... (Enter send, Ctrl+S save, Ctrl+X cancel, type exit to quit)"
	ta.Focus()
	ta.SetHeight(3)
	ta.SetWidth(80)
	ta.ShowLineNumbers = false
	ta.CharLimit = 4096

	renderer, _ := glamour.NewTermRenderer(
		glamour.WithStylePath("dark"),
		glamour.WithWordWrap(80),
	)

	sysPrompt := ai.ChatSystemPrompt(sysInfo.ContextBlock())

	m := Model{
		textarea: ta,
		messages: []chatMessage{},
		history: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: sysPrompt},
		},
		runner:      chatutil.NewRunner(client, sysInfo),
		sysInfo:     sysInfo,
		sessionName: opts.SessionName,
		renderer:    renderer,
	}

	loadedMessages, loadedHistory := loadChatState(opts.SessionName)
	if len(loadedMessages) > 0 {
		m.messages = append(m.messages, loadedMessages...)
	}
	if len(loadedHistory) > 0 {
		if loadedHistory[0].Role == openai.ChatMessageRoleSystem {
			m.history = loadedHistory
		} else {
			m.history = append(m.history[:1], loadedHistory...)
		}
		m.msgCount = len(m.messages)
	}

	return m
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, tea.EnterAltScreen)
}
