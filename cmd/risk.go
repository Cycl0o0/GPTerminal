package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/cycl0o0/GPTerminal/internal/risk"
	"github.com/spf13/cobra"
)

var riskCmd = &cobra.Command{
	Use:   "risk <command>",
	Short: "Evaluate the risk of a shell command",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		command := strings.Join(args, " ")
		fmt.Printf("Evaluating risk of: \033[1m%s\033[0m\n", command)

		result, err := risk.Evaluate(cmd.Context(), command)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
		risk.PrintResult(result)
	},
}

func init() {
	rootCmd.AddCommand(riskCmd)
}
