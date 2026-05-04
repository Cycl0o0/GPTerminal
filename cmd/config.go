package cmd

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"sort"
	"time"

	"github.com/cycl0o0/GPTerminal/internal/ai"
	"github.com/cycl0o0/GPTerminal/internal/config"
	"github.com/cycl0o0/GPTerminal/internal/usage"
	"github.com/spf13/cobra"
)

var (
	usageDaily  bool
	usageWeekly bool
	listModels  bool
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage GPTerminal configuration",
	Run: func(cmd *cobra.Command, args []string) {
		if listModels {
			client, err := ai.NewClient()
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				os.Exit(1)
			}
			ids, err := client.ListModels(context.Background())
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				os.Exit(1)
			}
			for _, id := range ids {
				fmt.Println(id)
			}
			return
		}
		cmd.Help()
	},
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
		fmt.Printf("Provider:           %s\n", config.ProviderName())
		fmt.Println()

		key := config.APIKey()
		if key != "" {
			fmt.Printf("API Key:            %s\n", maskKey(key))
		} else {
			fmt.Println("API Key:            (not set)")
		}
		if akey := config.AnthropicAPIKey(); akey != "" {
			fmt.Printf("Anthropic API Key:  %s\n", maskKey(akey))
		}
		if gkey := config.GeminiAPIKey(); gkey != "" {
			fmt.Printf("Gemini API Key:     %s\n", maskKey(gkey))
		}

		fmt.Printf("Base URL:           %s\n", config.APIBaseURL())
		fmt.Printf("Model:              %s\n", config.Model())
		fmt.Printf("Temperature:        %.1f\n", config.Temperature())
		fmt.Printf("Max Tokens:         %d\n", config.MaxTokens())
		fmt.Println()
		fmt.Printf("Image Model:        %s\n", config.ImageModel())
		fmt.Printf("Image Size:         %s\n", config.ImageSize())
		if u := config.ImageBaseURL(); u != config.APIBaseURL() {
			fmt.Printf("Image Base URL:     %s\n", u)
		}
		fmt.Println()
		fmt.Printf("S2T Model:          %s\n", config.S2TModel())
		if u := config.S2TBaseURL(); u != config.APIBaseURL() {
			fmt.Printf("S2T Base URL:       %s\n", u)
		}
		fmt.Printf("T2S Model:          %s\n", config.T2SModel())
		fmt.Printf("T2S Voice:          %s\n", config.T2SVoice())
		if u := config.T2SBaseURL(); u != config.APIBaseURL() {
			fmt.Printf("T2S Base URL:       %s\n", u)
		}
		fmt.Printf("Realtime Model:     %s\n", config.RealtimeModel())
		fmt.Printf("Realtime URL:       %s\n", config.RealtimeURL())
		fmt.Println()
		fmt.Printf("Config file:        %s\n", config.ConfigFile())

		if warnings := config.Validate(); len(warnings) > 0 {
			fmt.Fprintln(os.Stderr)
			for _, w := range warnings {
				fmt.Fprintf(os.Stderr, "Warning: %s\n", w)
			}
		}
	},
}

func maskKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "..." + key[len(key)-4:]
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

var modelsCmd = &cobra.Command{
	Use:   "models",
	Short: "List available models from the API",
	Run: func(cmd *cobra.Command, args []string) {
		client, err := ai.NewClient()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
		ids, err := client.ListModels(context.Background())
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
		for _, id := range ids {
			fmt.Println(id)
		}
	},
}

var setModelCmd = &cobra.Command{
	Use:   "set-model <model>",
	Short: "Set the model to use",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := config.SaveModel(args[0]); err != nil {
			fmt.Fprintln(os.Stderr, "Error saving model:", err)
			os.Exit(1)
		}
		fmt.Printf("Model set to %s\n", args[0])
	},
}

// ── per-service model commands ─────────────────────────────────────────────

func makeSetModelCmd(use, short, key string, saveFn func(string) error) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if err := saveFn(args[0]); err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				os.Exit(1)
			}
			fmt.Printf("Saved: %s = %s\n", key, args[0])
		},
	}
}

func makeSetURLCmd(use, short, key string, saveFn func(string) error) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			rawURL := args[0]
			u, err := url.ParseRequestURI(rawURL)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %q is not a valid URL: %v\n", rawURL, err)
				os.Exit(1)
			}
			if u.Scheme != "http" && u.Scheme != "https" && u.Scheme != "ws" && u.Scheme != "wss" {
				fmt.Fprintf(os.Stderr, "Error: %q must use http, https, ws, or wss scheme\n", rawURL)
				os.Exit(1)
			}
			if err := saveFn(rawURL); err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				os.Exit(1)
			}
			fmt.Printf("Saved: %s = %s\n", key, rawURL)
		},
	}
}

var setProviderCmd = &cobra.Command{
	Use:   "set-provider <provider>",
	Short: "Set the AI provider (openai, anthropic, gemini)",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		p := args[0]
		switch p {
		case "openai", "anthropic", "gemini":
		default:
			fmt.Fprintf(os.Stderr, "Unknown provider %q. Supported: openai, anthropic, gemini\n", p)
			os.Exit(1)
		}
		if err := config.SaveProvider(p); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
		fmt.Printf("Provider set to %s\n", p)
	},
}

var setAnthropicKeyCmd = &cobra.Command{
	Use:   "set-anthropic-key <api-key>",
	Short: "Save your Anthropic API key",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := config.SaveAnthropicAPIKey(args[0]); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
		fmt.Printf("Anthropic API key saved to %s\n", config.ConfigFile())
	},
}

var setGeminiKeyCmd = &cobra.Command{
	Use:   "set-gemini-key <api-key>",
	Short: "Save your Google Gemini API key",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := config.SaveGeminiAPIKey(args[0]); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
		fmt.Printf("Gemini API key saved to %s\n", config.ConfigFile())
	},
}

var (
	setS2TModelCmd    = makeSetModelCmd("set-s2t-model <model>", "Set speech-to-text model", "s2t_model", config.SaveS2TModel)
	setT2SModelCmd    = makeSetModelCmd("set-t2s-model <model>", "Set text-to-speech model", "t2s_model", config.SaveT2SModel)
	setT2SVoiceCmd    = makeSetModelCmd("set-t2s-voice <voice>", "Set text-to-speech voice", "t2s_voice", config.SaveT2SVoice)
	setImageModelCmd  = makeSetModelCmd("set-image-model <model>", "Set image generation model", "image_model", config.SaveImageModel)
	setRealtimeModelCmd = makeSetModelCmd("set-realtime-model <model>", "Set realtime transcription session model", "realtime_model", config.SaveRealtimeModel)
	setS2TURLCmd      = makeSetURLCmd("set-s2t-url <url>", "Set speech-to-text base URL", "s2t_base_url", config.SaveS2TBaseURL)
	setT2SURLCmd      = makeSetURLCmd("set-t2s-url <url>", "Set text-to-speech base URL", "t2s_base_url", config.SaveT2SBaseURL)
	setImageURLCmd    = makeSetURLCmd("set-image-url <url>", "Set image generation base URL", "image_base_url", config.SaveImageBaseURL)
	setRealtimeURLCmd = makeSetURLCmd("set-realtime-url <url>", "Set realtime WebSocket URL (wss://...)", "realtime_url", config.SaveRealtimeURL)
)

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
	configCmd.Flags().BoolVar(&listModels, "list-models", false, "List available models from the API")

	configCmd.AddCommand(setKeyCmd)
	configCmd.AddCommand(setBaseURLCmd)
	configCmd.AddCommand(setProviderCmd)
	configCmd.AddCommand(setAnthropicKeyCmd)
	configCmd.AddCommand(setGeminiKeyCmd)
	configCmd.AddCommand(showConfigCmd)
	configCmd.AddCommand(usageCmd)
	configCmd.AddCommand(modelsCmd)
	configCmd.AddCommand(setModelCmd)
	// per-service model commands
	configCmd.AddCommand(setS2TModelCmd)
	configCmd.AddCommand(setT2SModelCmd)
	configCmd.AddCommand(setT2SVoiceCmd)
	configCmd.AddCommand(setImageModelCmd)
	configCmd.AddCommand(setRealtimeModelCmd)
	// per-service URL commands
	configCmd.AddCommand(setS2TURLCmd)
	configCmd.AddCommand(setT2SURLCmd)
	configCmd.AddCommand(setImageURLCmd)
	configCmd.AddCommand(setRealtimeURLCmd)
	rootCmd.AddCommand(configCmd)
}
