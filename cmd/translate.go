package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/cycl0o0/GPTerminal/internal/translate"
	"github.com/cycl0o0/GPTerminal/internal/usage"
	"github.com/spf13/cobra"
)

var translateOutput string

var translateCmd = &cobra.Command{
	Use:   "translate <file> <target-language>",
	Short: "Translate source code to another programming language",
	Long:  "Translate a source file into another programming language with idiomatic output. The AI detects the source language automatically.",
	Example: "  gpterminal translate main.py go\n" +
		"  gpterminal translate server.js typescript\n" +
		"  gpterminal translate utils.go rust --output utils.rs",
	Args: cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		usage.Global().SetCurrentCommand("translate")
		targetLang := strings.Join(args[1:], " ")
		if err := translate.Run(cmd.Context(), args[0], targetLang, translateOutput); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
	},
}

func init() {
	translateCmd.Flags().StringVarP(&translateOutput, "output", "o", "", "Output file path (default: auto-detected)")
	rootCmd.AddCommand(translateCmd)
}
