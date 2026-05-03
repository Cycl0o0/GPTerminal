package imagine

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cycl0o0/GPTerminal/internal/ai"
	"github.com/cycl0o0/GPTerminal/internal/config"
)

// Result holds output from an image generation request.
type Result struct {
	FilePath      string
	RevisedPrompt string
}

// Generate creates images via the OpenAI images API and saves them to disk.
func Generate(ctx context.Context, prompt, model, size, outputDir string, n int) ([]Result, error) {
	client, err := ai.NewClientWithBaseURL(config.ImageBaseURL())
	if err != nil {
		return nil, err
	}

	images, err := client.CreateImage(ctx, prompt, model, size, n)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("create output dir: %w", err)
	}

	var results []Result
	for i, img := range images {
		data, err := base64.StdEncoding.DecodeString(img.B64JSON)
		if err != nil {
			return results, fmt.Errorf("decode image %d: %w", i+1, err)
		}

		filename := fmt.Sprintf("image_%d.png", i+1)
		if n == 1 {
			filename = "image.png"
		}
		outPath := filepath.Join(outputDir, filename)

		if err := os.WriteFile(outPath, data, 0644); err != nil {
			return results, fmt.Errorf("write image %d: %w", i+1, err)
		}

		results = append(results, Result{
			FilePath:      outPath,
			RevisedPrompt: img.RevisedPrompt,
		})
	}

	return results, nil
}
