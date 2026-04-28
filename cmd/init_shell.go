package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var initShellCmd = &cobra.Command{
	Use:   "init <bash|zsh|fish|powershell>",
	Short: "Print shell configuration for GPTerminal aliases",
	Long:  "Prints shell config to set up aliases. Add to your rc file:\n  Bash/Zsh:    eval \"$(gpterminal init bash)\"\n  Fish:        gpterminal init fish | source\n  PowerShell:  gpterminal init powershell | Invoke-Expression",
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
		case "powershell":
			fmt.Print(powershellInit)
		default:
			fmt.Fprintf(os.Stderr, "Unsupported shell: %s (use bash, zsh, fish, or powershell)\n", shell)
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
alias gptrun='gpterminal run'
alias gptedit='gpterminal edit'
alias gptreview='gpterminal review'
alias gptcommit='gpterminal commit'
alias gptresume='gpterminal resume'
alias gptexplaindiff='gpterminal explain-diff'
alias gptsessions='gpterminal sessions'
alias gpts2t='gpterminal s2t'
alias gptt2s='gpterminal t2s'
alias risk='gpterminal risk'
alias vibe='gpterminal vibe'
alias gptread='gpterminal read'
alias gptimagine='gpterminal imagine'
alias gptstats='gpterminal stats'
alias gptagent='gpterminal agent'

# Flush history after each command for reliable fix
export PROMPT_COMMAND="history -a;${PROMPT_COMMAND}"
`

const zshInit = `# GPTerminal shell integration
alias fuck='gpterminal fix'
alias gptchat='gpterminal chat'
alias gptdo='gpterminal gptdo'
alias gptrun='gpterminal run'
alias gptedit='gpterminal edit'
alias gptreview='gpterminal review'
alias gptcommit='gpterminal commit'
alias gptresume='gpterminal resume'
alias gptexplaindiff='gpterminal explain-diff'
alias gptsessions='gpterminal sessions'
alias gpts2t='gpterminal s2t'
alias gptt2s='gpterminal t2s'
alias risk='gpterminal risk'
alias vibe='gpterminal vibe'
alias gptread='gpterminal read'
alias gptimagine='gpterminal imagine'
alias gptstats='gpterminal stats'
alias gptagent='gpterminal agent'

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
function gptrun --description 'GPTerminal: generate and run one command'
    gpterminal run $argv
end
function gptedit --description 'GPTerminal: AI edit a file with diff approval'
    gpterminal edit $argv
end
function gptreview --description 'GPTerminal: review a file or diff'
    gpterminal review $argv
end
function gptcommit --description 'GPTerminal: generate a commit message'
    gpterminal commit $argv
end
function gptresume --description 'GPTerminal: resume a saved session'
    gpterminal resume $argv
end
function gptsessions --description 'GPTerminal: manage saved sessions'
    gpterminal sessions $argv
end
function gptexplaindiff --description 'GPTerminal: explain a git diff'
    gpterminal explain-diff $argv
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
function gptstats --description 'GPTerminal: usage statistics'
    gpterminal stats $argv
end
function gptagent --description 'GPTerminal: autonomous agent'
    gpterminal agent $argv
end
`

const powershellInit = `# GPTerminal shell integration
function fuck { gpterminal fix $args }
function gptchat { gpterminal chat $args }
function gptdo { gpterminal gptdo $args }
function gptrun { gpterminal run $args }
function gptedit { gpterminal edit $args }
function gptreview { gpterminal review $args }
function gptcommit { gpterminal commit $args }
function gptresume { gpterminal resume $args }
function gptexplaindiff { gpterminal explain-diff $args }
function gptsessions { gpterminal sessions $args }
function gpts2t { gpterminal s2t $args }
function gptt2s { gpterminal t2s $args }
function risk { gpterminal risk $args }
function vibe { gpterminal vibe $args }
function gptread { gpterminal read $args }
function gptimagine { gpterminal imagine $args }
function gptstats { gpterminal stats $args }
function gptagent { gpterminal agent $args }
`
