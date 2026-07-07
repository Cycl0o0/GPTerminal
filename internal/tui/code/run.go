package codetui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cycl0o0/GPTerminal/internal/ai"
	"github.com/cycl0o0/GPTerminal/internal/chatutil"
	"github.com/cycl0o0/GPTerminal/internal/mcp"
	"github.com/cycl0o0/GPTerminal/internal/system"
)

// Run launches the full-screen code TUI and blocks until the user quits.
func Run(ctx context.Context, opts Options, client *ai.Client, sysInfo system.SystemInfo, runner *chatutil.Runner, mcpReg *mcp.Registry) error {
	m := NewModel(client, sysInfo, runner, mcpReg, opts)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithContext(ctx))
	_, err := p.Run()
	return err
}
