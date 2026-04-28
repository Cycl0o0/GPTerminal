package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/cycl0o0/GPTerminal/internal/chatutil"
	"github.com/cycl0o0/GPTerminal/internal/template"
	"github.com/cycl0o0/GPTerminal/internal/usage"
	"github.com/spf13/cobra"
)

var templateCmd = &cobra.Command{
	Use:   "template",
	Short: "Manage custom prompt templates",
}

var templateListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available templates",
	Run: func(cmd *cobra.Command, args []string) {
		specs, err := template.LoadAll()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
		if len(specs) == 0 {
			fmt.Printf("No templates found in %s\n", template.TemplatesDir())
			fmt.Println("Create one with: gpterminal template create <name>")
			return
		}
		fmt.Println("Available templates:")
		for _, s := range specs {
			fmt.Printf("  %-20s %s\n", s.Name, s.Description)
		}
	},
}

var templateCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a starter template file",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := template.CreateStarter(args[0]); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
	},
}

func init() {
	templateCmd.AddCommand(templateListCmd)
	templateCmd.AddCommand(templateCreateCmd)
	rootCmd.AddCommand(templateCmd)
}

func registerTemplateCommands(root *cobra.Command) {
	specs, err := template.LoadAll()
	if err != nil {
		return
	}

	for _, spec := range specs {
		s := spec // capture
		cmd := &cobra.Command{
			Use:   s.Name + " <input...>",
			Short: s.Description,
			Args:  cobra.ArbitraryArgs,
			Run: func(cmd *cobra.Command, args []string) {
				usage.Global().SetCurrentCommand("template:" + s.Name)

				input := strings.Join(args, " ")

				if s.InputMode == "stdin" || s.InputMode == "args+stdin" {
					if chatutil.HasPipedStdin(os.Stdin) {
						stdinData, err := chatutil.ReadPipedStdin(os.Stdin)
						if err != nil {
							fmt.Fprintln(os.Stderr, "Error:", err)
							os.Exit(1)
						}
						input = chatutil.BuildUserMessage(input, stdinData)
					}
				}

				if strings.TrimSpace(input) == "" {
					fmt.Fprintln(os.Stderr, "Error: no input provided")
					os.Exit(1)
				}

				// Collect variable overrides from flags
				vars := make(map[string]string)
				for k := range s.Variables {
					if val, _ := cmd.Flags().GetString("var-" + k); val != "" {
						vars[k] = val
					}
				}

				if err := template.Execute(cmd.Context(), s, input, vars); err != nil {
					fmt.Fprintln(os.Stderr, "Error:", err)
					os.Exit(1)
				}
			},
		}

		for k, v := range s.Variables {
			cmd.Flags().String("var-"+k, v, fmt.Sprintf("Template variable: %s", k))
		}

		root.AddCommand(cmd)
	}
}
