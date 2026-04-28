package cmd

import (
	"fmt"
	"os"
	"strings"

	runpkg "github.com/cycl0o0/GPTerminal/internal/run"
	"github.com/cycl0o0/GPTerminal/internal/usage"
	"github.com/spf13/cobra"
)

var runAutoYes bool

var runCmd = &cobra.Command{
	Use:   "run <request>",
	Short: "Generate, review, and execute one AI-planned command",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		usage.Global().SetCurrentCommand("run")
		if err := runpkg.Run(cmd.Context(), strings.Join(args, " "), runAutoYes); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
	},
}

func init() {
	runCmd.Flags().BoolVarP(&runAutoYes, "yes", "y", false, "Auto-approve command execution without prompting")
	rootCmd.AddCommand(runCmd)
}
