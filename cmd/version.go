package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

const Version = "2.4.1"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version of GPTerminal",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("GPTerminal v%s\n", Version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
