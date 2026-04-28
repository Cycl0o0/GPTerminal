package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/cycl0o0/GPTerminal/internal/config"
	"github.com/cycl0o0/GPTerminal/internal/speech"
	"github.com/cycl0o0/GPTerminal/internal/usage"
	"github.com/spf13/cobra"
)

var (
	t2sModel        string
	t2sVoice        string
	t2sInstructions string
	t2sFormat       string
	t2sOutput       string
	t2sSpeed        float64
)

var t2sCmd = &cobra.Command{
	Use:   "t2s <text>",
	Short: "Convert text into speech audio",
	Long:  "Generate spoken audio from text using OpenAI text-to-speech models.",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		usage.Global().SetCurrentCommand("t2s")
		if !cmd.Flags().Changed("model") {
			t2sModel = config.T2SModel()
		}
		if !cmd.Flags().Changed("voice") {
			t2sVoice = config.T2SVoice()
		}
		format, err := speech.ParseSpeechFormat(t2sFormat)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}

		input := strings.Join(args, " ")
		fmt.Printf("Generating speech with \033[1m%s\033[0m...\n", t2sModel)

		result, err := speech.Synthesize(cmd.Context(), input, speech.SynthesisOptions{
			Model:        t2sModel,
			Voice:        t2sVoice,
			Instructions: t2sInstructions,
			Format:       format,
			OutputPath:   t2sOutput,
			Speed:        t2sSpeed,
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}

		fmt.Printf("\033[1;32mSaved:\033[0m %s\n", result.OutputPath)
	},
}

func init() {
	t2sCmd.Flags().StringVar(&t2sModel, "model", speech.DefaultSpeechModel, "Text-to-speech model")
	t2sCmd.Flags().StringVar(&t2sVoice, "voice", speech.DefaultSpeechVoice, "Voice name")
	t2sCmd.Flags().StringVar(&t2sInstructions, "instructions", "", "Optional speaking style instructions")
	t2sCmd.Flags().StringVar(&t2sFormat, "format", string(speech.DefaultSpeechFormat), "Audio format: mp3, opus, aac, flac, wav, pcm")
	t2sCmd.Flags().StringVar(&t2sOutput, "output", "", "Optional output audio file path")
	t2sCmd.Flags().Float64Var(&t2sSpeed, "speed", 1.0, "Speech speed")
	rootCmd.AddCommand(t2sCmd)
}
