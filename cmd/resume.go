package cmd

import (
	"fmt"
	"os"

	"github.com/cycl0o0/GPTerminal/internal/gptdo"
	"github.com/cycl0o0/GPTerminal/internal/session"
	"github.com/spf13/cobra"
)

var resumeExport bool

var resumeCmd = &cobra.Command{
	Use:   "resume <session>",
	Short: "Resume or export a saved chat or gptdo session",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		if resumeExport {
			data, err := session.Export(name)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				os.Exit(1)
			}
			fmt.Println(string(data))
			return
		}

		record, err := session.Load(name)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}

		switch record.Kind {
		case session.KindGptDo:
			if err := gptdo.Resume(cmd.Context(), name); err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				os.Exit(1)
			}
		case session.KindChat:
			if err := runChatCommand(cmd, nil, name, true); err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				os.Exit(1)
			}
		default:
			fmt.Fprintf(os.Stderr, "Error: unsupported session kind %q\n", record.Kind)
			os.Exit(1)
		}
	},
}

func init() {
	resumeCmd.Flags().BoolVar(&resumeExport, "export", false, "Print the saved session JSON instead of resuming it")
	rootCmd.AddCommand(resumeCmd)
}
