package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var initShellCmd = &cobra.Command{
	Use:   "init <bash|zsh|fish>",
	Short: "Print shell configuration for GPTerminal aliases",
	Long:  "Prints shell config to set up aliases. Add to your rc file:\n  Bash/Zsh: eval \"$(gpterminal init bash)\"\n  Fish:     gpterminal init fish | source",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		shell := args[0]
		switch shell {
		case "bash":
			fmt.Print(bashInit)
		case "zsh":
			fmt.Print(zshInit)
		case "fish":
			fmt.Print(fishInit)
		default:
			fmt.Fprintf(os.Stderr, "Unsupported shell: %s (use bash, zsh, or fish)\n", shell)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(initShellCmd)
}

const bashInit = `# GPTerminal shell integration
alias fuck='gpterminal fix'
alias gptchat='gpterminal chat'
alias gptdo='gpterminal gptdo'
alias gpts2t='gpterminal s2t'
alias gptt2s='gpterminal t2s'
alias risk='gpterminal risk'
alias vibe='gpterminal vibe'
alias gptread='gpterminal read'
alias gptimagine='gpterminal imagine'

# Flush history after each command for reliable fix
export PROMPT_COMMAND="history -a;${PROMPT_COMMAND}"
`

const zshInit = `# GPTerminal shell integration
alias fuck='gpterminal fix'
alias gptchat='gpterminal chat'
alias gptdo='gpterminal gptdo'
alias gpts2t='gpterminal s2t'
alias gptt2s='gpterminal t2s'
alias risk='gpterminal risk'
alias vibe='gpterminal vibe'
alias gptread='gpterminal read'
alias gptimagine='gpterminal imagine'

# Flush history after each command for reliable fix
setopt INC_APPEND_HISTORY
`

const fishInit = `# GPTerminal shell integration
function fuck --description 'GPTerminal: fix last command'
    gpterminal fix
end
function gptchat --description 'GPTerminal: AI chat'
    gpterminal chat
end
function gptdo --description 'GPTerminal: execute an AI command plan'
    gpterminal gptdo $argv
end
function gpts2t --description 'GPTerminal: speech to text'
    gpterminal s2t $argv
end
function gptt2s --description 'GPTerminal: text to speech'
    gpterminal t2s $argv
end
function risk --description 'GPTerminal: evaluate command risk'
    gpterminal risk $argv
end
function vibe --description 'GPTerminal: natural language to command'
    gpterminal vibe $argv
end
function gptread --description 'GPTerminal: analyze a file with AI'
    gpterminal read $argv
end
function gptimagine --description 'GPTerminal: generate an image'
    gpterminal imagine $argv
end
`
