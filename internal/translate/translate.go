package translate

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cycl0o0/GPTerminal/internal/ai"
	"github.com/cycl0o0/GPTerminal/internal/fileutil"
	"github.com/cycl0o0/GPTerminal/internal/system"
	openai "github.com/sashabaranov/go-openai"
)

var extMap = map[string]string{
	"go":         ".go",
	"golang":     ".go",
	"python":     ".py",
	"py":         ".py",
	"javascript": ".js",
	"js":         ".js",
	"typescript": ".ts",
	"ts":         ".ts",
	"rust":       ".rs",
	"rs":         ".rs",
	"ruby":       ".rb",
	"rb":         ".rb",
	"java":       ".java",
	"c":          ".c",
	"cpp":        ".cpp",
	"c++":        ".cpp",
	"csharp":     ".cs",
	"c#":         ".cs",
	"swift":      ".swift",
	"kotlin":     ".kt",
	"kt":         ".kt",
	"php":        ".php",
	"lua":        ".lua",
	"perl":       ".pl",
	"scala":      ".scala",
	"bash":       ".sh",
	"sh":         ".sh",
	"shell":      ".sh",
	"zig":        ".zig",
	"elixir":     ".ex",
	"haskell":    ".hs",
	"ocaml":      ".ml",
	"r":          ".r",
	"dart":       ".dart",
}

type response struct {
	SourceLang string `json:"source_lang"`
	TargetLang string `json:"target_lang"`
	Summary    string `json:"summary"`
	Content    string `json:"content"`
	Filename   string `json:"filename"`
}

func Run(ctx context.Context, path, targetLang string, outputPath string) error {
	client, err := ai.NewClient()
	if err != nil {
		return err
	}

	path = filepath.Clean(strings.TrimSpace(path))
	source, err := fileutil.ReadText(path)
	if err != nil {
		return fmt.Errorf("read source file: %w", err)
	}

	sysInfo := system.Detect()
	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: ai.TranslateSystemPrompt(sysInfo.ContextBlock())},
		{Role: openai.ChatMessageRoleUser, Content: fmt.Sprintf("Source file: %s\nTarget language: %s\n\nSource code:\n```\n%s\n```", path, targetLang, source)},
	}

	fmt.Fprintf(os.Stderr, "Translating %s to %s...", filepath.Base(path), targetLang)
	raw, err := client.Complete(ctx, messages)
	fmt.Fprintf(os.Stderr, "\r                                          \r")
	if err != nil {
		return err
	}

	result, err := parseResponse(raw)
	if err != nil {
		return err
	}

	if result.Summary != "" {
		fmt.Fprintf(os.Stderr, "\033[1m%s\033[0m\n", result.Summary)
	}
	fmt.Fprintf(os.Stderr, "\033[90m%s → %s\033[0m\n", result.SourceLang, result.TargetLang)

	if outputPath == "" {
		outputPath = suggestOutputPath(path, targetLang, result.Filename)
	}

	fmt.Println(result.Content)

	fmt.Fprintf(os.Stderr, "\nWrite to %s? [Y/n] ", outputPath)
	var answer string
	fmt.Scanln(&answer)
	answer = strings.TrimSpace(strings.ToLower(answer))
	if answer != "" && answer != "y" && answer != "yes" {
		fmt.Fprintln(os.Stderr, "Aborted.")
		return nil
	}

	if dir := filepath.Dir(outputPath); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}

	if err := fileutil.WriteText(outputPath, result.Content); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "\033[32mWrote %s\033[0m\n", outputPath)
	return nil
}

func suggestOutputPath(srcPath, targetLang, aiFilename string) string {
	if aiFilename != "" {
		return aiFilename
	}

	base := strings.TrimSuffix(filepath.Base(srcPath), filepath.Ext(srcPath))
	lang := strings.ToLower(strings.TrimSpace(targetLang))

	if ext, ok := extMap[lang]; ok {
		return base + ext
	}
	return base + "." + lang
}

func parseResponse(raw string) (*response, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start == -1 || end == -1 || end < start {
		return nil, fmt.Errorf("no JSON object found in translate response")
	}

	var resp response
	if err := json.Unmarshal([]byte(raw[start:end+1]), &resp); err != nil {
		return nil, err
	}
	if strings.TrimSpace(resp.Content) == "" {
		return nil, fmt.Errorf("empty translation result")
	}
	return &resp, nil
}
