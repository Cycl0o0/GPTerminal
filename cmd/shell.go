package cmd

import (
	"fmt"
	"os"

	"github.com/cycl0o0/GPTerminal/internal/aishell"
	"github.com/cycl0o0/GPTerminal/internal/usage"
	"github.com/spf13/cobra"
)

var shellCmd = &cobra.Command{
	Use:   "shell",
	Short: "AI-enhanced interactive shell",
	Long:  "Launch an interactive shell with AI superpowers. Failed commands get automatic fix suggestions, use ? to ask questions, and ! to generate commands from natural language.",
	Example: "  gpterminal shell\n" +
		"  # Inside the shell:\n" +
		"  ? how do I find large files\n" +
		"  ! list all docker containers sorted by size",
	Run: func(cmd *cobra.Command, args []string) {
		usage.Global().SetCurrentCommand("shell")
		if err := aishell.Run(cmd.Context()); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(shellCmd)
}
