package cmd

import (
	"fmt"
	"os"

	"github.com/cycl0o0/GPTerminal/internal/serve"
	"github.com/cycl0o0/GPTerminal/internal/usage"
	"github.com/spf13/cobra"
)

var serveStdio bool

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run a headless NDJSON agent server over stdio (for GUI clients)",
	Long: "Start a long-lived agent process that speaks newline-delimited JSON on stdin/stdout.\n" +
		"Requests (prompt, approval_response, cancel, config_get/set/list, sessions_list, models_list, shutdown)\n" +
		"are read from stdin; typed events (content, thinking, tool_call, tool_result, approval_request,\n" +
		"usage, done, error) are written to stdout. Human/diagnostic logging goes to stderr so it never\n" +
		"corrupts the protocol stream. This is the backend for the GPTerminal-GUI client.",
	Run: func(cmd *cobra.Command, args []string) {
		usage.Global().SetCurrentCommand("serve")
		srv := serve.New(serve.Options{Version: Version})
		if err := srv.Run(cmd.Context()); err != nil {
			fmt.Fprintln(os.Stderr, "serve:", err)
			os.Exit(1)
		}
	},
}

func init() {
	// --stdio is the only transport today; the flag exists so the invocation is
	// explicit and future transports (e.g. a socket) can be added without
	// changing the default behavior.
	serveCmd.Flags().BoolVar(&serveStdio, "stdio", true, "Communicate over stdin/stdout (NDJSON)")
	rootCmd.AddCommand(serveCmd)
}
