package cmd

import (
	"fmt"
	"os"

	"github.com/cycl0o0/GPTerminal/internal/config"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "gpterminal",
	Short: "GPTerminal - AI-powered terminal assistant",
	Long:  "GPTerminal seamlessly integrates OpenAI GPT or other OpenAI API-compatible models (like Ollama) into your Linux terminal.\nCommand correction, TUI chat, risk evaluation, and natural language commands.\n\nMade with <3 by Cycl0o0",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(config.Init)
	rootCmd.PersistentFlags().String("model", "", "OpenAI model to use")
	rootCmd.PersistentFlags().String("api-key", "", "OpenAI API key")
	rootCmd.PersistentFlags().String("api-base-url", "", "API base URL (e.g. http://localhost:11434/v1 for Ollama)")
}
