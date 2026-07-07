package codetui

import "github.com/charmbracelet/lipgloss"

// Palette matches the CLI's lipgloss ANSI-256 colors (and the GPTerminal-GUI
// glass theme): teal 86, blue 39, green 35, orange 214, red 196, purple 141,
// lilac 183, dim 243.
var (
	headerStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	headerDim   = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
	dividerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))

	userLabel  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	aiLabel    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("35"))
	sysLabel   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("141"))
	timestamp  = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	bodyStyle  = lipgloss.NewStyle().PaddingLeft(2)

	toolStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	toolOKStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("35"))
	thinkStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("183")).Italic(true)
	diffAddStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("35"))
	diffDelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	diffHunk     = lipgloss.NewStyle().Foreground(lipgloss.Color("141"))

	statusStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("243")).Padding(0, 1)
	streamingStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true).Padding(0, 1)
	errorStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true).Padding(0, 1)
	approvalStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true).Padding(0, 1)

	inputBorder = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("86")).Padding(0, 1)

	welcomeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("243")).Italic(true).Padding(1, 2)
)
