package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version is the GPTerminal version. It defaults to "dev" for local builds;
// release binaries override it via -ldflags "-X ...cmd.Version=<tag>" in the
// release workflow. It must be a var (not const) because the linker -X flag
// can only set string variables. Never edit this string to version a release
// — the tag + ldflags is the source of truth.
var Version = "dev"

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
