package cmd

import (
	"fmt"
	"os"

	"github.com/cycl0o0/GPTerminal/internal/commitmsg"
	"github.com/cycl0o0/GPTerminal/internal/usage"
	"github.com/spf13/cobra"
)

var commitConventional bool
var commitApply bool

var commitCmd = &cobra.Command{
	Use:   "commit",
	Short: "Generate a commit message from the staged diff",
	Run: func(cmd *cobra.Command, args []string) {
		usage.Global().SetCurrentCommand("commit")
		if err := commitmsg.Run(cmd.Context(), commitConventional, commitApply); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
	},
}

func init() {
	commitCmd.Flags().BoolVar(&commitConventional, "conventional", false, "Generate a conventional commit subject")
	commitCmd.Flags().BoolVar(&commitApply, "apply", false, "Run git commit with the generated message")
	rootCmd.AddCommand(commitCmd)
}
