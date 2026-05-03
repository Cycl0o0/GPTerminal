package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/cycl0o0/GPTerminal/internal/config"
	"github.com/cycl0o0/GPTerminal/internal/speech"
	"github.com/cycl0o0/GPTerminal/internal/usage"
	"github.com/spf13/cobra"
)

var (
	s2tModel         string
	s2tLanguage      string
	s2tPrompt        string
	s2tFormat        string
	s2tOutput        string
	s2tTranslate     bool
	s2tMic           bool
	s2tRecorder      string
	s2tDevice        string
	s2tRealtimeModel string
)

var s2tCmd = &cobra.Command{
	Use:   "s2t [audio-file]",
	Short: "Transcribe speech audio into text",
	Long: `Transcribe supported audio files into text using OpenAI speech-to-text models, or stream live transcription from your microphone with --mic. Microphone mode currently supports Linux only.

DISCLAIMER: You are solely responsible for the use of this transcription feature. Ensure you have proper authorization and consent before recording or transcribing any audio. Do not use this tool to transcribe conversations or audio without the knowledge and explicit consent of all parties involved. The authors of GPTerminal accept no liability for misuse.`,
	Args: func(cmd *cobra.Command, args []string) error {
		if s2tMic {
			if len(args) != 0 {
				return fmt.Errorf("do not pass an audio file when using --mic")
			}
			return nil
		}
		if len(args) != 1 {
			return fmt.Errorf("accepts exactly 1 audio file argument unless --mic is used")
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		usage.Global().SetCurrentCommand("s2t")
		if !cmd.Flags().Changed("model") {
			s2tModel = config.S2TModel()
		}
		format, err := speech.ParseTranscriptionFormat(s2tFormat)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}

		if !cmd.Flags().Changed("session-model") {
			s2tRealtimeModel = config.RealtimeModel()
		}
		if s2tMic {
			if s2tTranslate {
				fmt.Fprintln(os.Stderr, "Error: --translate is not supported with --mic realtime transcription")
				os.Exit(1)
			}
			if format != speech.DefaultTranscriptionFormat {
				fmt.Fprintln(os.Stderr, "Error: realtime microphone transcription currently supports only --format text")
				os.Exit(1)
			}

			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			fmt.Println("\033[1;33mDisclaimer:\033[0m You are solely responsible for the use of this feature. Ensure you have authorization and consent before recording or transcribing any audio.")
			fmt.Println("Listening from microphone. Press Ctrl+C to stop.")
			result, err := speech.TranscribeMicrophoneRealtime(ctx, speech.RealtimeTranscriptionOptions{
				SessionModel:       s2tRealtimeModel,
				TranscriptionModel: s2tModel,
				Language:           s2tLanguage,
				Prompt:             s2tPrompt,
				Recorder:           s2tRecorder,
				Device:             s2tDevice,
				OutputPath:         s2tOutput,
			})
			if err != nil && !errors.Is(err, context.Canceled) {
				fmt.Fprintln(os.Stderr, "Error:", err)
				os.Exit(1)
			}
			if result != nil && result.OutputPath != "" {
				fmt.Printf("\n\033[1;32mSaved:\033[0m %s\n", result.OutputPath)
			} else {
				fmt.Println()
			}
			return
		}

		audioPath := args[0]
		fmt.Printf("Transcribing: \033[1m%s\033[0m\n", audioPath)
		fmt.Print("Analyzing...")

		result, err := speech.Transcribe(cmd.Context(), audioPath, speech.TranscriptionOptions{
			Model:              s2tModel,
			Language:           s2tLanguage,
			Prompt:             s2tPrompt,
			Format:             format,
			TranslateToEnglish: s2tTranslate,
			OutputPath:         s2tOutput,
		})
		fmt.Print("\r            \r")
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}

		if result.OutputPath != "" {
			fmt.Printf("\033[1;32mSaved:\033[0m %s\n", result.OutputPath)
			return
		}

		fmt.Println(result.Content)
	},
}

func init() {
	s2tCmd.Flags().StringVar(&s2tModel, "model", speech.DefaultTranscriptionModel, "Speech-to-text model")
	s2tCmd.Flags().StringVar(&s2tLanguage, "language", "", "Optional source language hint (for transcriptions)")
	s2tCmd.Flags().StringVar(&s2tPrompt, "prompt", "", "Optional prompt to guide transcription")
	s2tCmd.Flags().StringVar(&s2tFormat, "format", string(speech.DefaultTranscriptionFormat), "Output format: text, json, verbose_json, srt, vtt")
	s2tCmd.Flags().StringVar(&s2tOutput, "output", "", "Optional output file path")
	s2tCmd.Flags().BoolVar(&s2tTranslate, "translate", false, "Translate speech to English instead of transcribing in the original language")
	s2tCmd.Flags().BoolVar(&s2tMic, "mic", false, "Use live microphone transcription instead of uploading an audio file (Linux only for now)")
	s2tCmd.Flags().StringVar(&s2tRecorder, "recorder", "auto", "Microphone recorder backend: auto, pw-record, parec, arecord, or ffmpeg")
	s2tCmd.Flags().StringVar(&s2tDevice, "device", "", "Optional microphone device/source for the selected recorder backend")
	s2tCmd.Flags().StringVar(&s2tRealtimeModel, "session-model", speech.DefaultRealtimeSessionModel, "Realtime session model for live microphone transcription")
	rootCmd.AddCommand(s2tCmd)
}
