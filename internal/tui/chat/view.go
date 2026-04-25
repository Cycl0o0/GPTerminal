package chat

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

func (m Model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	// Header bar
	title := headerStyle.Render("GPTerminal Chat")
	info := headerInfoStyle.Render(fmt.Sprintf("%s | %d messages", m.sysInfo.OS, m.msgCount))
	header := lipgloss.JoinHorizontal(lipgloss.Center, title, info)

	// Divider
	divider := dividerStyle.Render(strings.Repeat("─", m.width))

	// Status bar
	var status string
	if m.streaming {
		status = streamingStyle.Render("● AI is responding...")
	} else if m.err != nil {
		status = errorStyle.Render(fmt.Sprintf("✗ Error: %v", m.err))
	} else {
		status = statusStyle.Render("Enter send • Ctrl+N new chat • Ctrl+C quit")
	}

	// Input
	input := inputBorderStyle.Width(m.width - 4).Render(m.textarea.View())

	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		divider,
		m.viewport.View(),
		status,
		input,
	)
}

func (m Model) renderMessages() string {
	if len(m.messages) == 0 && !m.streaming {
		return welcomeStyle.Render("Welcome to GPTerminal Chat!\nAsk anything about your system, commands, or programming.")
	}

	var sb strings.Builder

	for _, msg := range m.messages {
		ts := ""
		if msg.timestamp != "" {
			ts = timestampStyle.Render(msg.timestamp)
		}

		switch msg.role {
		case "user":
			label := userLabelStyle.Render("You")
			sb.WriteString(fmt.Sprintf("%s %s\n", label, ts))
			sb.WriteString(userMsgStyle.Render(msg.content))
			sb.WriteString("\n\n")
		case "assistant":
			label := assistantLabelStyle.Render("GPTerminal")
			sb.WriteString(fmt.Sprintf("%s %s\n", label, ts))
			rendered := m.renderMarkdown(msg.content)
			sb.WriteString(assistantMsgStyle.Render(rendered))
			sb.WriteString("\n\n")
		}
	}

	// Show streaming buffer
	if m.streaming {
		label := assistantLabelStyle.Render("GPTerminal")
		if m.streamBuf != "" {
			sb.WriteString(fmt.Sprintf("%s\n", label))
			sb.WriteString(assistantMsgStyle.Render(m.streamBuf + "█"))
			sb.WriteString("\n")
		} else {
			sb.WriteString(fmt.Sprintf("%s %s\n", label, streamingStyle.Render("thinking...")))
		}
	}

	return sb.String()
}

func (m Model) renderMarkdown(content string) string {
	if m.renderer == nil {
		return content
	}
	rendered, err := m.renderer.Render(content)
	if err != nil {
		return content
	}
	return strings.TrimSpace(rendered)
}

func (m *Model) updateRenderer() {
	width := m.width - 6
	if width < 40 {
		width = 40
	}
	m.renderer, _ = glamour.NewTermRenderer(
		glamour.WithStylePath("dark"),
		glamour.WithWordWrap(width),
	)
}
