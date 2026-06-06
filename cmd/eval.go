package cmd

import (
	"fmt"
	"os"

	"github.com/cycl0o0/GPTerminal/internal/eval"
	"github.com/spf13/cobra"
)

var (
	evalJSON    bool
	evalFixture string
)

var evalCmd = &cobra.Command{
	Use:   "eval",
	Short: "Run the deterministic execution-policy regression suite",
	Long:  "Evaluate the local command-risk policy against a fixture of commands (no LLM, no shell). Exits non-zero if any case fails. Use --fixture to supply a custom JSON array of {name, command, expect} cases.",
	Run: func(cmd *cobra.Command, args []string) {
		cases := eval.DefaultCases()
		if evalFixture != "" {
			raw, err := os.ReadFile(evalFixture)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				os.Exit(2)
			}
			cases, err = eval.ParseFixture(raw)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				os.Exit(2)
			}
		}

		rep := eval.Run(cases)

		if evalJSON {
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
	evalCmd.Flags().BoolVar(&evalJSON, "json", false, "Emit the report as stable JSON")
	evalCmd.Flags().StringVar(&evalFixture, "fixture", "", "Path to a JSON fixture of policy cases")
	rootCmd.AddCommand(evalCmd)
}
