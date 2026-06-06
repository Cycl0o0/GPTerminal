package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/cycl0o0/GPTerminal/internal/gptdo"
	"github.com/cycl0o0/GPTerminal/internal/usage"
	"github.com/spf13/cobra"
)

var (
	gptdoSession string
	gptdoYes     bool
	gptdoJSON    bool
)

var gptdoCmd = &cobra.Command{
	Use:     "gptdo <request>",
	Aliases: []string{"do"},
	Short:   "Let AI execute an approved sequence of shell commands",
	Args:    cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		usage.Global().SetCurrentCommand("gptdo")
		request := strings.Join(args, " ")
		if err := gptdo.Run(cmd.Context(), request, gptdoSession, gptdoYes, gptdoJSON); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
	},
}

func init() {
	gptdoCmd.Flags().StringVar(&gptdoSession, "session", "", "Save progress to a named session for later resume")
	// --yes auto-approves Allowed and NeedsConfirm commands. It NEVER bypasses a
	// Denied command (INSTRUCTIONS.md §5).
	gptdoCmd.Flags().BoolVarP(&gptdoYes, "yes", "y", false, "Auto-approve non-denied commands (does not bypass Denied)")
	// --json: non-interactive, emits a stable RunReport to stdout (human output
	// goes to stderr). Allowed commands run; NeedsConfirm requires --yes; Denied
	// never runs.
	gptdoCmd.Flags().BoolVar(&gptdoJSON, "json", false, "Emit a stable JSON run report (non-interactive)")
	rootCmd.AddCommand(gptdoCmd)
}
