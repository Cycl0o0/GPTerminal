package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate shell completion scripts",
	Long: `Generate shell completion scripts for GPTerminal.

Bash:
  $ gpterminal completion bash > /etc/bash_completion.d/gpterminal
  # or for current user
  $ gpterminal completion bash > ~/.local/share/bash-completion/completions/gpterminal

Zsh:
  $ gpterminal completion zsh > "${fpath[1]}/_gpterminal"
  # You may need to restart your shell or run: compinit

Fish:
  $ gpterminal completion fish > ~/.config/fish/completions/gpterminal.fish

PowerShell:
  PS> gpterminal completion powershell | Out-String | Invoke-Expression
  # To load on startup, add the above to your PowerShell profile.`,
	Args:      cobra.ExactArgs(1),
	ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
	Run: func(cmd *cobra.Command, args []string) {
		switch args[0] {
		case "bash":
			rootCmd.GenBashCompletion(os.Stdout)
		case "zsh":
			rootCmd.GenZshCompletion(os.Stdout)
		case "fish":
			rootCmd.GenFishCompletion(os.Stdout, true)
		case "powershell":
			rootCmd.GenPowerShellCompletionWithDesc(os.Stdout)
		}
	},
}

func init() {
	rootCmd.AddCommand(completionCmd)
}
