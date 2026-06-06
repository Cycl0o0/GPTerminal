package cmd

import (
	"fmt"
	"os"

	"github.com/cycl0o0/GPTerminal/internal/doctor"
	"github.com/spf13/cobra"
)

var doctorJSON bool

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnose configuration and security posture",
	Long:  "Run offline health checks: provider/credential, model, config file, execution policy, redaction, MCP allowlist, and tool availability. Exits non-zero if a required check fails.",
	Run: func(cmd *cobra.Command, args []string) {
		rep := doctor.Run()

		if doctorJSON {
			out, err := rep.JSON()
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				os.Exit(2)
			}
			fmt.Println(out)
		} else {
			fmt.Print(rep.Text())
		}

		if !rep.OK {
			os.Exit(1)
		}
	},
}

func init() {
	doctorCmd.Flags().BoolVar(&doctorJSON, "json", false, "Emit the report as stable JSON")
	rootCmd.AddCommand(doctorCmd)
}
