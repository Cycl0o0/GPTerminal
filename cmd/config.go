package cmd

import (
	"fmt"
	"net/url"
	"os"
	"sort"
	"time"

	"github.com/cycl0o0/GPTerminal/internal/config"
	"github.com/cycl0o0/GPTerminal/internal/usage"
	"github.com/spf13/cobra"
)

var (
	usageDaily  bool
	usageWeekly bool
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
		rawURL := args[0]

		// Validate URL format before saving
		u, err := url.ParseRequestURI(rawURL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %q is not a valid URL: %v\n", rawURL, err)
			os.Exit(1)
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			fmt.Fprintf(os.Stderr, "Error: %q must use http or https scheme\n", rawURL)
			os.Exit(1)
		}

		if err := config.SaveAPIBaseURL(rawURL); err != nil {
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

		// Print validation warnings
		if warnings := config.Validate(); len(warnings) > 0 {
			fmt.Fprintln(os.Stderr)
			for _, w := range warnings {
				fmt.Fprintf(os.Stderr, "Warning: %s\n", w)
			}
		}
	},
}

var usageCmd = &cobra.Command{
	Use:   "usage",
	Short: "Show API usage and cost for the current month",
	Run: func(cmd *cobra.Command, args []string) {
		u := usage.Global().CurrentUsage()

		if usageDaily {
			keys := sortedKeys(u.DailyCosts)
			if len(keys) == 0 {
				fmt.Println("No daily cost data recorded yet.")
				return
			}
			for _, day := range keys {
				fmt.Printf("%s: $%.4f\n", day, u.DailyCosts[day])
			}
			return
		}

		if usageWeekly {
			keys := sortedKeys(u.DailyCosts)
			if len(keys) == 0 {
				fmt.Println("No daily cost data recorded yet.")
				return
			}
			weeks := map[string]float64{}
			for _, day := range keys {
				t, err := time.Parse("2006-01-02", day)
				if err != nil {
					continue
				}
				year, week := t.ISOWeek()
				key := fmt.Sprintf("%d-W%02d", year, week)
				weeks[key] += u.DailyCosts[day]
			}
			weekKeys := sortedKeys(weeks)
			for _, wk := range weekKeys {
				fmt.Printf("%s: $%.4f\n", wk, weeks[wk])
			}
			return
		}

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

func sortedKeys(m map[string]float64) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func init() {
	usageCmd.Flags().BoolVar(&usageDaily, "daily", false, "Show daily cost breakdown")
	usageCmd.Flags().BoolVar(&usageWeekly, "weekly", false, "Show weekly cost breakdown")

	configCmd.AddCommand(setKeyCmd)
	configCmd.AddCommand(setBaseURLCmd)
	configCmd.AddCommand(showConfigCmd)
	configCmd.AddCommand(usageCmd)
	rootCmd.AddCommand(configCmd)
}
