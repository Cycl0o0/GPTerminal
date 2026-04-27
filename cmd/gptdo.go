package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/cycl0o0/GPTerminal/internal/gptdo"
	"github.com/spf13/cobra"
)

var gptdoCmd = &cobra.Command{
	Use:   "gptdo <request>",
	Short: "Let AI execute an approved sequence of shell commands",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		request := strings.Join(args, " ")
		if err := gptdo.Run(cmd.Context(), request); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(gptdoCmd)
}
