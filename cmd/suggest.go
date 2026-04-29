package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/cycl0o0/GPTerminal/internal/suggest"
	"github.com/cycl0o0/GPTerminal/internal/usage"
	"github.com/spf13/cobra"
)

var suggestCmd = &cobra.Command{
	Use:   "suggest <buffer>",
	Short: "Complete or correct a shell command inline",
	Args:  cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		usage.Global().SetCurrentCommand("suggest")
		buffer := strings.Join(args, " ")

		if strings.TrimSpace(buffer) == "" {
			return
		}

		if err := suggest.Run(cmd.Context(), buffer); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(suggestCmd)
}
