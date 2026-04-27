package cmd

import (
	"fmt"
	"os"
	"strings"

	runpkg "github.com/cycl0o0/GPTerminal/internal/run"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run <request>",
	Short: "Generate, review, and execute one AI-planned command",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := runpkg.Run(cmd.Context(), strings.Join(args, " ")); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
}
