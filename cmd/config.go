package cmd

import (
	"fmt"
	"os"

	"github.com/cycl0o0/GPTerminal/internal/config"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage GPTerminal configuration",
}

var setKeyCmd = &cobra.Command{
	Use:   "set-key <api-key>",
	Short: "Save your OpenAI API key to config",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		key := args[0]
		if err := config.SaveAPIKey(key); err != nil {
			fmt.Fprintln(os.Stderr, "Error saving API key:", err)
			os.Exit(1)
		}
		fmt.Printf("API key saved to %s\n", config.ConfigFile())
	},
}

var showConfigCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	Run: func(cmd *cobra.Command, args []string) {
		key := config.APIKey()
		if key != "" {
			// Mask the key for security
			masked := key[:4] + "..." + key[len(key)-4:]
			fmt.Printf("API Key: %s\n", masked)
		} else {
			fmt.Println("API Key: (not set)")
		}
		fmt.Printf("Model: %s\n", config.Model())
		fmt.Printf("Temperature: %.1f\n", config.Temperature())
		fmt.Printf("Max Tokens: %d\n", config.MaxTokens())
		fmt.Printf("Config file: %s\n", config.ConfigFile())
	},
}

func init() {
	configCmd.AddCommand(setKeyCmd)
	configCmd.AddCommand(showConfigCmd)
	rootCmd.AddCommand(configCmd)
}
