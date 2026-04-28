package stats

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("12")).
			Padding(0, 1)

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("15"))

	valueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("10"))

	tableHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("14"))

	barStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("12"))

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8"))
)

func (m Model) View() string {
	d := m.dashboard
	var b strings.Builder

	b.WriteString(titleStyle.Render(fmt.Sprintf("GPTerminal Dashboard — %s", d.Month)))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", min(m.width, 60)))
	b.WriteString("\n\n")

	b.WriteString(headerStyle.Render("Total Cost:   "))
	b.WriteString(valueStyle.Render(fmt.Sprintf("$%.4f", d.TotalCost)))
	b.WriteString("   ")
	b.WriteString(headerStyle.Render("Calls: "))
	b.WriteString(valueStyle.Render(fmt.Sprintf("%d", d.TotalCalls)))
	b.WriteString("   ")
	b.WriteString(headerStyle.Render("Tokens: "))
	b.WriteString(valueStyle.Render(fmt.Sprintf("%d", d.TotalTokens)))
	b.WriteString("\n\n")

	if len(d.TopCommands) > 0 {
		b.WriteString(headerStyle.Render("Top Commands"))
		b.WriteString("\n")
		b.WriteString(tableHeaderStyle.Render(fmt.Sprintf("  %-16s %6s %10s %10s", "COMMAND", "CALLS", "COST", "TOKENS")))
		b.WriteString("\n")
		for _, cs := range d.TopCommands {
			b.WriteString(fmt.Sprintf("  %-16s %6d %10s %10d\n", cs.Name, cs.Count, fmt.Sprintf("$%.4f", cs.Cost), cs.Tokens))
		}
		b.WriteString("\n")
	}

	if len(d.DailyTrend) > 0 {
		b.WriteString(headerStyle.Render("Daily Costs"))
		b.WriteString("\n")
		maxCost := 0.0
		for _, e := range d.DailyTrend {
			if e.Cost > maxCost {
				maxCost = e.Cost
			}
		}
		barWidth := 25
		for _, e := range d.DailyTrend {
			bars := 0
			if maxCost > 0 {
				bars = int((e.Cost / maxCost) * float64(barWidth))
			}
			if bars < 1 && e.Cost > 0 {
				bars = 1
			}
			b.WriteString(fmt.Sprintf("  %s %s $%.4f\n", e.Date, barStyle.Render(strings.Repeat("█", bars)), e.Cost))
		}
		b.WriteString("\n")
	}

	b.WriteString(helpStyle.Render("Press q to quit"))
	return b.String()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
