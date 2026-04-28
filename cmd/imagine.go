package cmd

import (
	"fmt"
	"os"

	"github.com/cycl0o0/GPTerminal/internal/config"
	"github.com/cycl0o0/GPTerminal/internal/imagine"
	"github.com/cycl0o0/GPTerminal/internal/usage"
	"github.com/spf13/cobra"
)

var imagineCmd = &cobra.Command{
	Use:   "imagine <prompt>",
	Short: "Generate an image from a text prompt",
	Long:  "Generate images using OpenAI's image generation API (DALL-E / GPT-Image).",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		usage.Global().SetCurrentCommand("imagine")
		prompt := args[0]
		if len(args) > 1 {
			// Join all args as the prompt for convenience
			prompt = ""
			for i, a := range args {
				if i > 0 {
					prompt += " "
				}
				prompt += a
			}
		}

		model, _ := cmd.Flags().GetString("model")
		size, _ := cmd.Flags().GetString("size")
		n, _ := cmd.Flags().GetInt("n")
		output, _ := cmd.Flags().GetString("output")

		fmt.Printf("Generating %d image(s) with \033[1m%s\033[0m...\n", n, model)

		results, err := imagine.Generate(cmd.Context(), prompt, model, size, output, n)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}

		for _, r := range results {
			fmt.Printf("\033[1;32mSaved:\033[0m %s\n", r.FilePath)
			if r.RevisedPrompt != "" {
				fmt.Printf("\033[1;36mRevised prompt:\033[0m %s\n", r.RevisedPrompt)
			}
		}
	},
}

func init() {
	imagineCmd.Flags().String("model", config.DefaultImageModel, "Image model (dall-e-2, dall-e-3, gpt-image-1)")
	imagineCmd.Flags().String("size", config.DefaultImageSize, "Image size (e.g. 1024x1024, 1024x1792)")
	imagineCmd.Flags().Int("n", 1, "Number of images to generate")
	imagineCmd.Flags().String("output", ".", "Output directory for generated images")
	rootCmd.AddCommand(imagineCmd)
}
