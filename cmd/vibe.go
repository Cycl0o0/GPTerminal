package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/cycl0o0/GPTerminal/internal/chatutil"
	"github.com/cycl0o0/GPTerminal/internal/usage"
	"github.com/cycl0o0/GPTerminal/internal/vibe"
	"github.com/spf13/cobra"
)

var vibeAutoYes bool

var vibeCmd = &cobra.Command{
	Use:   "vibe <description>",
	Short: "Translate natural language into a shell command",
	Args:  cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		usage.Global().SetCurrentCommand("vibe")
		description := strings.Join(args, " ")

		if chatutil.HasPipedStdin(os.Stdin) {
			stdinData, err := chatutil.ReadPipedStdin(os.Stdin)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				os.Exit(1)
			}
			description = chatutil.BuildUserMessage(description, stdinData)
		}

		if strings.TrimSpace(description) == "" {
			fmt.Fprintln(os.Stderr, "Error: no description provided")
			os.Exit(1)
		}

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
