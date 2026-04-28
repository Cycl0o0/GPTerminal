package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/cycl0o0/GPTerminal/internal/chatutil"
	"github.com/cycl0o0/GPTerminal/internal/reader"
	"github.com/cycl0o0/GPTerminal/internal/usage"
	"github.com/spf13/cobra"
)

var readCmd = &cobra.Command{
	Use:   "read [file] [question...]",
	Short: "Analyze a file or piped text with AI",
	Long:  "Read a file, URL, or piped text and ask AI a question about it.\nSupports text files, images (png/jpg/gif/webp), PDFs, and remote URLs.",
	Example: "  gpterminal read ./server.log \"summarize the main errors\"\n" +
		"  gpterminal read ./diagram.png \"describe this image\"\n" +
		"  gpterminal read https://example.com/docs \"summarize this page\"\n" +
		"  cat server.log | gpterminal read \"summarize the main failures\"",
	Run: func(cmd *cobra.Command, args []string) {
		usage.Global().SetCurrentCommand("read")
		if chatutil.HasPipedStdin(os.Stdin) {
			stdinData, err := chatutil.ReadPipedStdin(os.Stdin)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				os.Exit(1)
			}
			if strings.TrimSpace(stdinData) == "" {
				fmt.Fprintln(os.Stderr, "Error: no stdin input provided")
				os.Exit(1)
			}

			question := strings.TrimSpace(strings.Join(args, " "))
			fmt.Println("Reading text input from stdin")
			fmt.Print("Analyzing...")

			result, err := reader.ReadTextInput(cmd.Context(), "stdin", stdinData, question)
			fmt.Print("\r            \r")
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				os.Exit(1)
			}

			fmt.Println(result)
			return
		}

		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Error: usage: gpterminal read <file> <question...>")
			os.Exit(1)
		}

		filePath := args[0]
		question := strings.Join(args[1:], " ")

		if reader.IsURL(filePath) {
			fmt.Printf("Reading URL: \033[1m%s\033[0m\n", filePath)
			fmt.Print("Analyzing...")

			result, err := reader.ReadURL(cmd.Context(), filePath, question)
			fmt.Print("\r            \r")
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				os.Exit(1)
			}

			fmt.Println(result)
			return
		}

		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "File not found: %s\n", filePath)
			os.Exit(1)
		}

		kind := reader.DetectKind(filePath)
		fmt.Printf("Reading %s file: \033[1m%s\033[0m\n", kind, filePath)
		fmt.Print("Analyzing...")

		result, err := reader.ReadFile(cmd.Context(), filePath, question)
		fmt.Print("\r            \r")
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}

		fmt.Println(result)
	},
}

func init() {
	rootCmd.AddCommand(readCmd)
}
