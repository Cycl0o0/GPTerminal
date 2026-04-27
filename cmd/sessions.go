package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/cycl0o0/GPTerminal/internal/session"
	"github.com/spf13/cobra"
)

var sessionsJSON bool
var sessionsMarkdown bool
var sessionsForce bool

var sessionsCmd = &cobra.Command{
	Use:   "sessions",
	Short: "List and manage saved chat and gptdo sessions",
	Run: func(cmd *cobra.Command, args []string) {
		if err := runSessionsList(); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
	},
}

var sessionsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List saved sessions",
	Run: func(cmd *cobra.Command, args []string) {
		if err := runSessionsList(); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
	},
}

var sessionsShowCmd = &cobra.Command{
	Use:   "show <session>",
	Short: "Inspect a saved session",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		record, err := session.Load(args[0])
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}

		if sessionsJSON {
			data, err := session.Export(args[0])
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				os.Exit(1)
			}
			fmt.Println(string(data))
			return
		}

		if sessionsMarkdown {
			md, err := session.ExportMarkdown(args[0])
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				os.Exit(1)
			}
			fmt.Print(md)
			return
		}

		printSessionRecord(record)
	},
}

var sessionsRenameCmd = &cobra.Command{
	Use:   "rename <old> <new>",
	Short: "Rename a saved session",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		if err := session.Rename(args[0], args[1]); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
		fmt.Printf("Renamed session %q to %q\n", session.NormalizeName(args[0]), session.NormalizeName(args[1]))
	},
}

var sessionsDeleteCmd = &cobra.Command{
	Use:     "delete <session>",
	Aliases: []string{"rm", "remove"},
	Short:   "Delete a saved session",
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := session.NormalizeName(args[0])
		if name == "" {
			fmt.Fprintln(os.Stderr, "Error: session name cannot be empty")
			os.Exit(1)
		}

		if !sessionsForce {
			fmt.Printf("Delete session %q? [y/N] ", name)
			var answer string
			fmt.Scanln(&answer)
			answer = strings.TrimSpace(strings.ToLower(answer))
			if answer != "y" && answer != "yes" {
				fmt.Println("Aborted.")
				return
			}
		}

		if err := session.Delete(name); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
		fmt.Printf("Deleted session %q\n", name)
	},
}

func init() {
	sessionsShowCmd.Flags().BoolVar(&sessionsJSON, "json", false, "Print the raw saved session JSON")
	sessionsShowCmd.Flags().BoolVar(&sessionsMarkdown, "markdown", false, "Export session as readable markdown")
	sessionsDeleteCmd.Flags().BoolVar(&sessionsForce, "force", false, "Delete without confirmation")

	sessionsCmd.AddCommand(sessionsListCmd, sessionsShowCmd, sessionsRenameCmd, sessionsDeleteCmd)
	rootCmd.AddCommand(sessionsCmd)
}

func runSessionsList() error {
	entries, err := session.List()
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		fmt.Println("No saved sessions.")
		return nil
	}

	fmt.Printf("%-20s %-6s %-16s %s\n", "NAME", "KIND", "UPDATED", "DETAIL")
	for _, entry := range entries {
		fmt.Printf("%-20s %-6s %-16s %s\n",
			entry.Name,
			strings.ToUpper(string(entry.Kind)),
			formatSessionTime(entry.UpdatedAt),
			sessionDetail(entry),
		)
	}
	return nil
}

func printSessionRecord(record *session.Record) {
	fmt.Printf("Name: %s\n", record.Name)
	fmt.Printf("Kind: %s\n", strings.ToUpper(string(record.Kind)))
	fmt.Printf("Updated: %s\n", record.UpdatedAt.Local().Format(time.RFC1123))

	switch record.Kind {
	case session.KindChat:
		if record.Chat == nil {
			return
		}
		fmt.Printf("Messages: %d\n", len(record.Chat.Transcript))
		if len(record.Chat.Transcript) > 0 {
			last := record.Chat.Transcript[len(record.Chat.Transcript)-1]
			fmt.Printf("Last message role: %s\n", last.Role)
			fmt.Printf("Last preview: %s\n", previewText(last.Content))
		}
	case session.KindGptDo:
		if record.GptDo == nil {
			return
		}
		fmt.Printf("Completed: %t\n", record.GptDo.Completed)
		fmt.Printf("Working directory: %s\n", record.GptDo.CWD)
		fmt.Printf("Auto-approve: %t\n", record.GptDo.AutoApprove)
		fmt.Printf("Request: %s\n", record.GptDo.Request)
		if strings.TrimSpace(record.GptDo.Summary) != "" {
			fmt.Printf("Summary: %s\n", record.GptDo.Summary)
		}
	}
}

func sessionDetail(entry session.Entry) string {
	switch entry.Kind {
	case session.KindChat:
		detail := fmt.Sprintf("%d messages", entry.ChatMessages)
		if entry.LastPreview != "" {
			detail += " | " + entry.LastPreview
		}
		return detail
	case session.KindGptDo:
		status := "in progress"
		if entry.GptDoCompleted {
			status = "completed"
		}
		detail := status
		if entry.GptDoRequest != "" {
			detail += " | " + entry.GptDoRequest
		} else if entry.GptDoSummary != "" {
			detail += " | " + entry.GptDoSummary
		}
		return detail
	default:
		return ""
	}
}

func formatSessionTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Local().Format("2006-01-02 15:04")
}

func previewText(s string) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	if len(s) > 120 {
		return s[:120] + "..."
	}
	return s
}
