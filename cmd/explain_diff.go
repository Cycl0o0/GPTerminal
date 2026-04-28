package cmd

import (
	"fmt"
	"os"

	"github.com/cycl0o0/GPTerminal/internal/chatutil"
	"github.com/cycl0o0/GPTerminal/internal/explaindiff"
	"github.com/cycl0o0/GPTerminal/internal/usage"
	"github.com/spf13/cobra"
)

var explainDiffStaged bool

var explainDiffCmd = &cobra.Command{
	Use:   "explain-diff [path...]",
	Short: "Explain the current git diff in plain language",
	Run: func(cmd *cobra.Command, args []string) {
		usage.Global().SetCurrentCommand("explain-diff")
		stdinDiff := ""
		if chatutil.HasPipedStdin(os.Stdin) {
			var err error
			stdinDiff, err = chatutil.ReadPipedStdin(os.Stdin)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				os.Exit(1)
			}
		}

		out, err := explaindiff.Run(cmd.Context(), explainDiffStaged, args, stdinDiff)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
		fmt.Println(out)
	},
}

func init() {
	explainDiffCmd.Flags().BoolVar(&explainDiffStaged, "staged", false, "Explain the staged git diff")
	rootCmd.AddCommand(explainDiffCmd)
}
