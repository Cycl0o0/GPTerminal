package chat

import (
	"context"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/cycl0o0/GPTerminal/internal/ai"
	openai "github.com/sashabaranov/go-openai"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			// Save history on exit
			_ = saveHistory(m.messages)
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
			m.streaming = false
			if m.ready {
				m.viewport.SetContent(m.renderMessages())
			}
			_ = saveHistory(m.messages)
			return m, nil

		case tea.KeyEnter:
			if m.streaming {
				break
			}
			input := m.textarea.Value()
			if input == "" {
				break
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
			m.err = nil

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
		m.streamBuf += msg.Content
		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoBottom()
		return m, nil

	case StreamDoneMsg:
		m.streaming = false
		m.messages = append(m.messages, chatMessage{
			role:      "assistant",
			content:   msg.FullContent,
			timestamp: nowTimestamp(),
		})
		m.history = append(m.history, openai.ChatCompletionMessage{
			Role: openai.ChatMessageRoleAssistant, Content: msg.FullContent,
		})
		m.msgCount++
		m.streamBuf = ""
		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoBottom()

		// Auto-save after each response
		_ = saveHistory(m.messages)
		return m, nil

	case StreamErrMsg:
		m.streaming = false
		m.err = msg.Err
		m.streamBuf = ""
		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoBottom()
		return m, nil
	}

	// Update textarea
	if !m.streaming {
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

func (m *Model) startStream() tea.Cmd {
	history := make([]openai.ChatCompletionMessage, len(m.history))
	copy(history, m.history)
	client := m.client

	return func() tea.Msg {
		full, err := client.StreamComplete(context.Background(), history, func(chunk string) {
			// Chunks are accumulated in StreamComplete
		})
		if err != nil {
			return StreamErrMsg{Err: err}
		}
		return StreamDoneMsg{FullContent: full}
	}
}
