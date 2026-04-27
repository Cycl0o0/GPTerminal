package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/cycl0o0/GPTerminal/internal/reader"
	"github.com/spf13/cobra"
)

var readCmd = &cobra.Command{
	Use:   "read <file> <question...>",
	Short: "Analyze a file (text or image) with AI",
	Long:  "Read a file and ask AI a question about it.\nSupports text files, images (png/jpg/gif/webp), and PDFs.",
	Args:  cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		filePath := args[0]
		question := strings.Join(args[1:], " ")

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
