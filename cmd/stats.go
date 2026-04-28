package cmd

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cycl0o0/GPTerminal/internal/stats"
	tuistats "github.com/cycl0o0/GPTerminal/internal/tui/stats"
	"github.com/cycl0o0/GPTerminal/internal/usage"
	"github.com/spf13/cobra"
)

var statsTUI bool

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show usage statistics and cost dashboard",
	Run: func(cmd *cobra.Command, args []string) {
		data := usage.Global().CurrentUsage()
		dashboard := stats.BuildDashboard(data)

		if statsTUI {
			model := tuistats.NewModel(dashboard)
			p := tea.NewProgram(model, tea.WithAltScreen())
			if _, err := p.Run(); err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				os.Exit(1)
			}
			return
		}

		stats.PrintPlain(dashboard)
	},
}

func init() {
	statsCmd.Flags().BoolVar(&statsTUI, "tui", false, "Show interactive TUI dashboard")
	rootCmd.AddCommand(statsCmd)
}
