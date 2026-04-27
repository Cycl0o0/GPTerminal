package cmd

import (
	"fmt"
	"os"

	"github.com/cycl0o0/GPTerminal/internal/review"
	"github.com/spf13/cobra"
)

var reviewStaged bool

var reviewCmd = &cobra.Command{
	Use:   "review [file]",
	Short: "Review a file or git diff for bugs, risks, and missing tests",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		path := ""
		if len(args) == 1 {
			path = args[0]
		}
		out, err := review.Run(cmd.Context(), path, reviewStaged)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
		fmt.Println(out)
	},
}

func init() {
	reviewCmd.Flags().BoolVar(&reviewStaged, "staged", false, "Review the staged git diff instead of the working tree diff")
	rootCmd.AddCommand(reviewCmd)
}
