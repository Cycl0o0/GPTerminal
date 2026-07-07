// Package codetui is the full-screen bubbletea TUI for `gpterminal code`: a
// Claude Code-style coding agent surface with a streaming transcript, a tool
// timeline, an inline diff-rendered approval modal with hotkeys, slash
// commands, and a model/effort/approval status line.
//
// It reuses the proven channel-bridge pattern from internal/tui/chat (a
// buffered chan of tea.Msg fed by the runner's callbacks, drained by a
// self-re-arming waitForEvent Cmd) but fixes that model's two goroutine leaks:
// approvals select on ctx.Done() so a cancel unblocks them, and starting a new
// turn / cancelling always cancels the prior stream and stops draining safely.
package codetui

import (
	"context"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/cycl0o0/GPTerminal/internal/ai"
	"github.com/cycl0o0/GPTerminal/internal/chatutil"
	"github.com/cycl0o0/GPTerminal/internal/mcp"
	"github.com/cycl0o0/GPTerminal/internal/session"
	"github.com/cycl0o0/GPTerminal/internal/system"
	openai "github.com/sashabaranov/go-openai"
)

// Options configures the code TUI.
type Options struct {
	SessionName  string
	ApprovalMode chatutil.ApprovalMode
	ProjectCtx   string
	Provider     string
}

// entryKind classifies a transcript entry for rendering.
type entryKind int

const (
	entryUser entryKind = iota
	entryAI
	entryThinking
	entryTool
	entrySystem
	entryError
)

type transcriptEntry struct {
	kind    entryKind
	label   string
	body    string
	ts      string
	toolOK  bool // for entryTool: whether the result was a success
	toolRes string
}

type approvalState struct {
	kind      string
	command   string
	path      string
	diff      string
	risk      string
	allowAuto bool
}

// Model is the bubbletea model for the code TUI.
type Model struct {
	runner   *chatutil.Runner
	client   *ai.Client
	sysInfo  system.SystemInfo
	mcp      *mcp.Registry
	opts     Options
	renderer *glamour.TermRenderer

	viewport viewport.Model
	textarea textarea.Model

	entries []transcriptEntry
	history []openai.ChatCompletionMessage

	width, height int
	ready         bool

	// streaming state
	streaming    bool
	streamBuf    string // accumulating assistant text for the live block
	thinkingBuf  string
	statusLine   string
	err          error
	autoApprove  bool
	pending      *approvalState

	// bridge
	eventCh      chan tea.Msg
	approvalCh   chan chatutil.ApprovalDecision
	streamCancel context.CancelFunc

	sessionName string
	transcript  []session.ChatMessage
	quitting    bool
}

// NewModel builds the code TUI model, seeding history from an existing session
// when one is named.
func NewModel(client *ai.Client, sysInfo system.SystemInfo, runner *chatutil.Runner, mcpReg *mcp.Registry, opts Options) Model {
	ta := textarea.New()
	ta.Placeholder = "Ask the agent to build, fix, or explain… (Enter send · Ctrl-J newline · Esc cancel · Ctrl-C quit)"
	ta.Focus()
	ta.CharLimit = 0
	ta.ShowLineNumbers = false
	ta.SetHeight(3)
	ta.SetWidth(80)

	renderer, _ := glamour.NewTermRenderer(glamour.WithStylePath("dark"), glamour.WithWordWrap(80))

	m := Model{
		runner:      runner,
		client:      client,
		sysInfo:     sysInfo,
		mcp:         mcpReg,
		opts:        opts,
		renderer:    renderer,
		textarea:    ta,
		sessionName: opts.SessionName,
	}

	// Seed system prompt (unless OpenClaw manages history server-side).
	if !client.IsOpenClaw() {
		m.history = []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: ai.CodeSystemPrompt(sysInfo.ContextBlock(), opts.ProjectCtx)},
		}
	}

	if opts.SessionName != "" {
		if rec, err := session.Load(opts.SessionName); err == nil && rec.Chat != nil {
			if len(rec.Chat.History) > 0 {
				m.history = rec.Chat.History
			}
			m.transcript = append(m.transcript, rec.Chat.Transcript...)
			for _, msg := range rec.Chat.Transcript {
				m.entries = append(m.entries, transcriptFromSaved(msg))
			}
		}
	}

	return m
}

func transcriptFromSaved(msg session.ChatMessage) transcriptEntry {
	kind := entryAI
	label := "agent"
	switch msg.Role {
	case openai.ChatMessageRoleUser:
		kind, label = entryUser, "you"
	case openai.ChatMessageRoleAssistant:
		kind, label = entryAI, "agent"
	default:
		kind, label = entrySystem, msg.Role
	}
	return transcriptEntry{kind: kind, label: label, body: msg.Content, ts: msg.Timestamp}
}

// Init starts the TUI.
func (m Model) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, tea.EnterAltScreen)
}
