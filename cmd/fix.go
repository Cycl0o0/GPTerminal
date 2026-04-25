package cmd

import (
	"fmt"
	"os"

	"github.com/cycl0o0/GPTerminal/internal/fix"
	"github.com/spf13/cobra"
)

var fixCmd = &cobra.Command{
	Use:     "fix",
	Aliases: []string{"fuck"},
	Short:   "Fix the last failed command using AI",
	Run: func(cmd *cobra.Command, args []string) {
		if err := fix.Run(cmd.Context()); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(fixCmd)
}
