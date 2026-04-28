package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/cycl0o0/GPTerminal/internal/chatutil"
	"github.com/cycl0o0/GPTerminal/internal/risk"
	"github.com/cycl0o0/GPTerminal/internal/usage"
	"github.com/spf13/cobra"
)

var riskCmd = &cobra.Command{
	Use:   "risk <command>",
	Short: "Evaluate the risk of a shell command",
	Args:  cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		usage.Global().SetCurrentCommand("risk")
		command := strings.Join(args, " ")

		if command == "" && chatutil.HasPipedStdin(os.Stdin) {
			stdinData, err := chatutil.ReadPipedStdin(os.Stdin)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				os.Exit(1)
			}
			command = strings.TrimSpace(stdinData)
		}

		if command == "" {
			fmt.Fprintln(os.Stderr, "Error: no command provided")
			os.Exit(1)
		}

		isTTY := chatutil.IsStdoutTTY()
		if isTTY {
			fmt.Printf("Evaluating risk of: \033[1m%s\033[0m\n", command)
		} else {
			fmt.Printf("Evaluating risk of: %s\n", command)
		}

		result, err := risk.Evaluate(cmd.Context(), command)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}

		if isTTY {
			risk.PrintResult(result)
		} else {
			risk.PrintResultPlain(result)
		}
	},
}

func init() {
	rootCmd.AddCommand(riskCmd)
}
