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
	info := headerInfoStyle.Render(m.headerInfo())
	header := lipgloss.JoinHorizontal(lipgloss.Center, title, info)

	// Divider
	divider := dividerStyle.Render(strings.Repeat("─", m.width))

	// Status bar
	var status string
	if m.pendingApproval != nil {
		status = statusStyle.Render(approvalStatus(m.pendingApproval))
	} else if m.streaming && m.thinkingText != "" {
		status = streamingStyle.Render(compactLine("Thinking: " + m.thinkingText))
	} else if m.streaming {
		status = streamingStyle.Render("● AI is responding... Ctrl+X to cancel")
	} else if m.err != nil {
		status = errorStyle.Render(fmt.Sprintf("✗ Error: %v", m.err))
	} else if m.notice != "" {
		status = statusStyle.Render(m.notice)
	} else {
		status = statusStyle.Render("Enter send • Ctrl+S save session • Ctrl+N new chat • Ctrl+C quit")
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
		sessionLine := "Session mode: ad-hoc"
		if m.sessionName != "" {
			sessionLine = "Session mode: " + m.sessionName
		}
		return welcomeStyle.Render("Welcome to GPTerminal Chat!\nAsk about your system, code, or commands.\n" + sessionLine + "\nShortcuts: Ctrl+S save session, Ctrl+N new chat, Ctrl+C quit.")
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
		} else if m.pendingApproval != nil {
			sb.WriteString(fmt.Sprintf("%s\n", label))
			sb.WriteString(assistantMsgStyle.Render(renderApprovalPrompt(m.pendingApproval)))
			sb.WriteString("\n")
		} else if m.thinkingText != "" {
			sb.WriteString(fmt.Sprintf("%s\n", label))
			sb.WriteString(assistantMsgStyle.Render(compactLine(m.thinkingText) + " █"))
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

func approvalStatus(p *approvalPrompt) string {
	if p == nil {
		return ""
	}
	if p.allowAuto {
		return "Approval required: type yes, auto, or no"
	}
	return "Approval required: type yes or no"
}

func renderApprovalPrompt(p *approvalPrompt) string {
	if p == nil {
		return ""
	}

	var b strings.Builder
	switch p.kind {
	case "command":
		b.WriteString("Approval required for command:\n")
		b.WriteString("```text\n")
		b.WriteString(p.command)
		b.WriteString("\n```\n")
		if p.risk != "" {
			b.WriteString("\n")
			b.WriteString(p.risk)
			b.WriteString("\n")
		}
	case "write_file":
		b.WriteString("Approval required for file write: `")
		b.WriteString(p.path)
		b.WriteString("`\n\n```diff\n")
		b.WriteString(p.diff)
		b.WriteString("\n```\n")
	}

	if p.allowAuto {
		b.WriteString("\nType `yes`, `auto`, or `no`.")
	} else {
		b.WriteString("\nType `yes` or `no`.")
	}
	return b.String()
}

func compactLine(s string) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	if len(s) > 120 {
		return s[:120] + "..."
	}
	return s
}

func (m Model) headerInfo() string {
	parts := []string{m.sysInfo.OS}
	if m.sessionName != "" {
		parts = append(parts, "session:"+m.sessionName)
	} else {
		parts = append(parts, "session:ad-hoc")
	}
	if m.autoApproveCommands {
		parts = append(parts, "auto-approve:on")
	}
	parts = append(parts, fmt.Sprintf("%d messages", m.msgCount))
	if m.sysInfo.WorkDir != "" {
		parts = append(parts, compactLine(m.sysInfo.WorkDir))
	}
	return strings.Join(parts, " | ")
}
