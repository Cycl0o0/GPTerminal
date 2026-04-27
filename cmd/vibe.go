package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/cycl0o0/GPTerminal/internal/vibe"
	"github.com/spf13/cobra"
)

var vibeAutoYes bool

var vibeCmd = &cobra.Command{
	Use:   "vibe <description>",
	Short: "Translate natural language into a shell command",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		description := strings.Join(args, " ")
		if err := vibe.Run(cmd.Context(), description, vibeAutoYes); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
	},
}

func init() {
	vibeCmd.Flags().BoolVarP(&vibeAutoYes, "yes", "y", false, "Auto-approve and execute the generated command")
	rootCmd.AddCommand(vibeCmd)
}
