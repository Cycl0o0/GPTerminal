package cmd

import (
	"fmt"
	"os"

	"github.com/cycl0o0/GPTerminal/internal/code"
	"github.com/cycl0o0/GPTerminal/internal/usage"
	"github.com/spf13/cobra"
)

var codeSession string

var codeCmd = &cobra.Command{
	Use:   "code",
	Short: "Interactive AI coding assistant",
	Long:  "Launch an interactive coding session with AI-powered file editing, code exploration, and project-aware assistance. Similar to Claude Code.",
	Example: "  gpterminal code\n" +
		"  gpterminal code --session myproject",
	Run: func(cmd *cobra.Command, args []string) {
		usage.Global().SetCurrentCommand("code")
		cfg := code.Config{
			SessionName: codeSession,
		}
		if err := code.Run(cmd.Context(), cfg); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
	},
}

func init() {
	codeCmd.Flags().StringVar(&codeSession, "session", "", "Use a named session for persistence")
	rootCmd.AddCommand(codeCmd)
}
