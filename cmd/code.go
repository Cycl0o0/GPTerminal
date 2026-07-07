package cmd

import (
	"fmt"
	"os"

	"github.com/cycl0o0/GPTerminal/internal/code"
	"github.com/cycl0o0/GPTerminal/internal/usage"
	"github.com/spf13/cobra"
)

var codeSession string
var codeModel string
var codeApproval string
var codeEffort string
var codeNoTUI bool

var codeCmd = &cobra.Command{
	Use:   "code",
	Short: "Interactive AI coding assistant",
	Long:  "Launch an interactive coding session with AI-powered file editing, code exploration, and project-aware assistance.",
	Example: "  gpterminal code\n" +
		"  gpterminal code --session myproject\n" +
		"  gpterminal code --model gpt-4o --approval auto-edit --effort high",
	Run: func(cmd *cobra.Command, args []string) {
		usage.Global().SetCurrentCommand("code")
		cfg := code.Config{
			SessionName:  codeSession,
			Model:        codeModel,
			ApprovalMode: codeApproval,
			Effort:       codeEffort,
			ForceNoTUI:   codeNoTUI,
		}
		if err := code.Run(cmd.Context(), cfg); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
	},
}

func init() {
	codeCmd.Flags().StringVar(&codeSession, "session", "", "Use a named session for persistence")
	codeCmd.Flags().StringVar(&codeModel, "model", "", "Override the model for this session")
	codeCmd.Flags().StringVar(&codeApproval, "approval", "", "Approval mode: plan | default | auto-edit | yolo")
	codeCmd.Flags().StringVar(&codeEffort, "effort", "", "Reasoning effort: none | minimal | low | medium | high | max")
	codeCmd.Flags().BoolVar(&codeNoTUI, "no-tui", false, "Use the plain REPL instead of the full-screen TUI")
	rootCmd.AddCommand(codeCmd)
}
