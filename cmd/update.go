package cmd

import (
	"fmt"
	"os"

	"github.com/cycl0o0/GPTerminal/internal/update"
	"github.com/spf13/cobra"
)

var updateCheck bool

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update GPTerminal to the latest version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Checking for updates...")
		result, err := update.Check(Version)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}

		if !result.Available {
			fmt.Printf("GPTerminal v%s is already up to date.\n", result.Current)
			return
		}

		fmt.Printf("Update available: v%s → v%s\n", result.Current, result.Latest)
		if result.Notes != "" {
			fmt.Printf("\nRelease notes:\n%s\n", result.Notes)
		}

		if updateCheck {
			return
		}

		if result.DownloadURL == "" {
			fmt.Fprintln(os.Stderr, "Error: no binary available for your platform")
			os.Exit(1)
		}

		fmt.Println("\nDownloading update...")
		if err := update.Apply(result); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}

		fmt.Printf("Successfully updated to v%s!\n", result.Latest)
	},
}

func init() {
	updateCmd.Flags().BoolVar(&updateCheck, "check", false, "Check for updates without installing")
	rootCmd.AddCommand(updateCmd)
}
