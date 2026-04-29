package cmd

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cycl0o0/GPTerminal/internal/tui/setup"
	"github.com/spf13/cobra"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Run the interactive setup wizard",
	Long:  "Launch a full-screen setup wizard to configure your API key, base URL, model, and shell integration.",
	Run: func(cmd *cobra.Command, args []string) {
		if err := runSetupWizard(); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(setupCmd)
}

func runSetupWizard() error {
	p := tea.NewProgram(setup.NewModel(), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
