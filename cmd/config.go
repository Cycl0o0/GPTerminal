package cmd

import (
	"fmt"
	"os"

	"github.com/cycl0o0/GPTerminal/internal/config"
	"github.com/cycl0o0/GPTerminal/internal/usage"
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

var setBaseURLCmd = &cobra.Command{
	Use:   "set-base-url <url>",
	Short: "Save the API base URL to config (e.g. http://localhost:11434/v1 for Ollama)",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		url := args[0]
		if err := config.SaveAPIBaseURL(url); err != nil {
			fmt.Fprintln(os.Stderr, "Error saving API base URL:", err)
			os.Exit(1)
		}
		fmt.Printf("API base URL saved to %s\n", config.ConfigFile())
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
		fmt.Printf("Base URL: %s\n", config.APIBaseURL())
		fmt.Printf("Model: %s\n", config.Model())
		fmt.Printf("Temperature: %.1f\n", config.Temperature())
		fmt.Printf("Max Tokens: %d\n", config.MaxTokens())
		fmt.Printf("Config file: %s\n", config.ConfigFile())
	},
}

var usageCmd = &cobra.Command{
	Use:   "usage",
	Short: "Show API usage and cost for the current month",
	Run: func(cmd *cobra.Command, args []string) {
		u := usage.Global().CurrentUsage()
		fmt.Printf("Month:           %s\n", u.Month)
		fmt.Printf("API Calls:       %d\n", u.Calls)
		fmt.Printf("Input Tokens:    %d\n", u.InputTokens)
		fmt.Printf("Output Tokens:   %d\n", u.OutputTokens)
		fmt.Printf("Images Generated:%d\n", u.ImagesGen)
		fmt.Printf("Chat Cost:       $%.4f\n", u.TotalCost-u.ImageCost)
		fmt.Printf("Image Cost:      $%.4f\n", u.ImageCost)
		fmt.Printf("Total Cost:      $%.4f\n", u.TotalCost)

		limit := config.CostLimit()
		if limit > 0 {
			pct := (u.TotalCost / limit) * 100
			fmt.Printf("Budget Limit:    $%.2f (%.1f%% used)\n", limit, pct)
		} else {
			fmt.Println("Budget Limit:    unlimited")
		}
	},
}

func init() {
	configCmd.AddCommand(setKeyCmd)
	configCmd.AddCommand(setBaseURLCmd)
	configCmd.AddCommand(showConfigCmd)
	configCmd.AddCommand(usageCmd)
	rootCmd.AddCommand(configCmd)
}
