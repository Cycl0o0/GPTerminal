package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/cycl0o0/GPTerminal/internal/edit"
	"github.com/cycl0o0/GPTerminal/internal/usage"
	"github.com/spf13/cobra"
)

var editCmd = &cobra.Command{
	Use:   "edit <file> <instruction...>",
	Short: "Ask AI to modify a file, preview the diff, and approve the write",
	Args:  cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		usage.Global().SetCurrentCommand("edit")
		if err := edit.Run(cmd.Context(), args[0], strings.Join(args[1:], " ")); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(editCmd)
}
