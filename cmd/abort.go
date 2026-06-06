package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/cycl0o0/GPTerminal/internal/session"
	"github.com/spf13/cobra"
)

var abortYes bool

var abortCmd = &cobra.Command{
	Use:   "abort <session>",
	Short: "Abort a saved session and remove its durable state",
	Long:  "Delete a saved chat/gptdo/agent/code session so it can no longer be resumed. Aborting only removes GPTerminal's own saved state; it never executes commands or touches your files.",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]

		// Confirm the session exists and show what is being removed.
		record, err := session.Load(name)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}

		fmt.Printf("Session: %s (kind: %s)\n", record.Name, record.Kind)
		if !abortYes && !confirmAbort() {
			fmt.Println("Aborted cancelled; session kept.")
			return
		}

		if err := session.Delete(name); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
		fmt.Printf("Removed saved session %q. It can no longer be resumed.\n", record.Name)
	},
}

func confirmAbort() bool {
	fmt.Print("Permanently delete this saved session? [y/N] ")
	answer, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))
	return answer == "y" || answer == "yes"
}

func init() {
	abortCmd.Flags().BoolVarP(&abortYes, "yes", "y", false, "Skip the confirmation prompt")
	rootCmd.AddCommand(abortCmd)
}
