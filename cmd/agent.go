package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/cycl0o0/GPTerminal/internal/agent"
	"github.com/cycl0o0/GPTerminal/internal/usage"
	"github.com/spf13/cobra"
)

var agentSession string
var agentMaxSteps int

var agentCmd = &cobra.Command{
	Use:   "agent <objective>",
	Short: "Autonomous AI agent that plans and executes tasks",
	Long:  "Launch an autonomous agent that plans and executes multi-step tasks using available tools (read_file, list_directory, search_text, run_command, write_file).",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		usage.Global().SetCurrentCommand("agent")
		objective := strings.Join(args, " ")
		cfg := agent.Config{
			Objective:   objective,
			SessionName: agentSession,
			MaxSteps:    agentMaxSteps,
		}
		if err := agent.Run(cmd.Context(), cfg); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
	},
}

func init() {
	agentCmd.Flags().StringVar(&agentSession, "session", "", "Save progress to a named session for later resume")
	agentCmd.Flags().IntVar(&agentMaxSteps, "max-steps", 50, "Maximum number of agent steps")
	rootCmd.AddCommand(agentCmd)
}
