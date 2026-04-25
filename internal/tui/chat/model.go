package chat

import (
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/cycl0o0/GPTerminal/internal/ai"
	"github.com/cycl0o0/GPTerminal/internal/system"
	openai "github.com/sashabaranov/go-openai"
)

type chatMessage struct {
	role      string
	content   string
	timestamp string
}

type Model struct {
	viewport  viewport.Model
	textarea  textarea.Model
	messages  []chatMessage
	history   []openai.ChatCompletionMessage
	client    *ai.Client
	sysInfo   system.SystemInfo
	streaming bool
	streamBuf string
	err       error
	width     int
	height    int
	ready     bool
	renderer  *glamour.TermRenderer
	msgCount  int
}

func NewModel(client *ai.Client, sysInfo system.SystemInfo) Model {
	ta := textarea.New()
	ta.Placeholder = "Ask anything... (Enter to send, Ctrl+C to quit, Ctrl+N new chat)"
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
		client:   client,
		sysInfo:  sysInfo,
		renderer: renderer,
	}

	// Load previous chat history
	if saved := loadHistory(); len(saved) > 0 {
		for _, s := range saved {
			m.messages = append(m.messages, chatMessage{
				role:      s.Role,
				content:   s.Content,
				timestamp: s.Timestamp,
			})
			m.history = append(m.history, openai.ChatCompletionMessage{
				Role:    s.Role,
				Content: s.Content,
			})
		}
		m.msgCount = len(m.messages)
	}

	return m
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, tea.EnterAltScreen)
}
