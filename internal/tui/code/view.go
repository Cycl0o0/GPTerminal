package codetui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/cycl0o0/GPTerminal/internal/config"
)

func newViewport(width, height int) viewport.Model {
	vp := viewport.New(width, height)
	vp.SetContent("")
	return vp
}

func (m *Model) updateRenderer(wrap int) {
	if wrap < 40 {
		wrap = 40
	}
	if r, err := glamour.NewTermRenderer(glamour.WithStylePath("dark"), glamour.WithWordWrap(wrap)); err == nil {
		m.renderer = r
	}
}

// View renders the full screen.
func (m Model) View() string {
	if !m.ready {
		return "Starting GPTCode…"
	}
	header := m.renderHeader()
	divider := dividerStyle.Render(strings.Repeat("─", max(m.width, 1)))
	status := m.renderStatus()
	input := inputBorder.Width(max(m.width-2, 10)).Render(m.textarea.View())
	return lipgloss.JoinVertical(lipgloss.Left, header, divider, m.viewport.View(), status, input)
}

func (m Model) renderHeader() string {
	left := headerStyle.Render("GPTCode")
	mode := string(m.opts.effectiveMode())
	eff := config.Effort()
	info := fmt.Sprintf("%s · %s · approval:%s · effort:%s", m.opts.Provider, config.Model(), mode, eff)
	if m.sessionName != "" {
		info += " · session:" + m.sessionName
	}
	if m.autoApprove {
		info += " · auto-approve:on"
	}
	right := headerDim.Render(info)
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, left, strings.Repeat(" ", gap), right)
}

func (m Model) renderStatus() string {
	switch {
	case m.pending != nil:
		return approvalStyle.Render(m.approvalHint())
	case m.streaming && m.statusLine != "":
		return streamingStyle.Render("● " + m.statusLine + "  (Esc to cancel)")
	case m.streaming:
		return streamingStyle.Render("● working…  (Esc to cancel)")
	case m.err != nil:
		return errorStyle.Render("error: " + m.err.Error())
	default:
		return statusStyle.Render("Enter send · Ctrl-J newline · /help · Ctrl-C quit")
	}
}

func (m Model) approvalHint() string {
	if m.pending.allowAuto {
		return "Approve? [y]es · [a]uto · [n]o"
	}
	return "Approve? [y]es · [n]o"
}

// refreshViewport re-renders the transcript into the viewport and pins to bottom.
func (m *Model) refreshViewport() {
	if !m.ready {
		return
	}
	var b strings.Builder
	if len(m.entries) == 0 && m.streamBuf == "" {
		b.WriteString(welcomeStyle.Render("Describe a coding task. The agent will explore, edit, run, and verify — you approve mutations."))
	}
	for i, e := range m.entries {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(m.renderEntry(e))
		b.WriteString("\n")
	}
	// Live streaming block.
	if m.streaming {
		if strings.TrimSpace(m.thinkingBuf) != "" {
			b.WriteString(thinkStyle.Render("⟡ "+firstLine(m.thinkingBuf, 400)) + "\n")
		}
		if m.streamBuf != "" {
			b.WriteString(aiLabel.Render("agent") + "\n")
			b.WriteString(bodyStyle.Render(m.renderMarkdown(m.streamBuf) + "▋"))
			b.WriteString("\n")
		}
	}
	// Approval modal rendered inline at the bottom of the transcript.
	if m.pending != nil {
		b.WriteString("\n")
		b.WriteString(m.renderApproval())
	}
	m.viewport.SetContent(b.String())
	m.viewport.GotoBottom()
}

func (m Model) renderEntry(e transcriptEntry) string {
	switch e.kind {
	case entryUser:
		head := userLabel.Render("you")
		if e.ts != "" {
			head += " " + timestamp.Render(e.ts)
		}
		return head + "\n" + bodyStyle.Render(e.body)
	case entryAI:
		head := aiLabel.Render("agent")
		if e.ts != "" {
			head += " " + timestamp.Render(e.ts)
		}
		return head + "\n" + bodyStyle.Render(m.renderMarkdown(e.body))
	case entryThinking:
		return thinkStyle.Render("⟡ " + e.body)
	case entryTool:
		icon := toolStyle.Render("⚡ " + e.label)
		if e.toolRes != "" {
			mark := toolOKStyle.Render("✓")
			if !e.toolOK {
				mark = diffDelStyle.Render("✗")
			}
			icon = fmt.Sprintf("%s %s %s", mark, toolStyle.Render(e.label), headerDim.Render(e.toolRes))
		} else if e.body != "" {
			icon += " " + headerDim.Render(e.body)
		}
		return icon
	case entrySystem:
		return sysLabel.Render("• ") + thinkStyle.Render(e.body)
	case entryError:
		return errorStyle.Render("error: " + e.body)
	}
	return e.body
}

func (m Model) renderApproval() string {
	var b strings.Builder
	if m.pending.kind == "command" {
		b.WriteString(approvalStyle.Render("Command approval") + "\n")
		b.WriteString(bodyStyle.Render("$ "+m.pending.command) + "\n")
		if m.pending.risk != "" {
			b.WriteString(bodyStyle.Render("risk: "+m.pending.risk) + "\n")
		}
	} else {
		b.WriteString(approvalStyle.Render("File write: "+m.pending.path) + "\n")
		b.WriteString(m.renderDiff(m.pending.diff))
	}
	b.WriteString(approvalStyle.Render(m.approvalHint()))
	return b.String()
}

// renderDiff colorizes a unified diff.
func (m Model) renderDiff(diff string) string {
	var b strings.Builder
	for _, line := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
			b.WriteString(diffAddStyle.Render(line))
		case strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---"):
			b.WriteString(diffDelStyle.Render(line))
		case strings.HasPrefix(line, "@@"):
			b.WriteString(diffHunk.Render(line))
		default:
			b.WriteString(headerDim.Render(line))
		}
		b.WriteString("\n")
	}
	return b.String()
}

func (m Model) renderMarkdown(s string) string {
	if m.renderer == nil {
		return s
	}
	out, err := m.renderer.Render(s)
	if err != nil {
		return s
	}
	return strings.TrimRight(out, "\n")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
