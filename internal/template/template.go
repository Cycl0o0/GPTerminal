package template

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cycl0o0/GPTerminal/internal/ai"
	"github.com/cycl0o0/GPTerminal/internal/chatutil"
	"github.com/cycl0o0/GPTerminal/internal/system"
	openai "github.com/sashabaranov/go-openai"
	"go.yaml.in/yaml/v3"
)

type Spec struct {
	Name         string            `yaml:"name"`
	Description  string            `yaml:"description"`
	SystemPrompt string            `yaml:"system_prompt"`
	InputMode    string            `yaml:"input_mode"` // "args", "stdin", "file", "args+stdin"
	Variables    map[string]string `yaml:"variables"`
	Stream       bool              `yaml:"stream"`
	UseTools     bool              `yaml:"use_tools"`
}

func TemplatesDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".config", "gpterminal", "templates")
}

func LoadAll() ([]Spec, error) {
	dir := TemplatesDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var specs []Spec
	for _, entry := range entries {
		ext := filepath.Ext(entry.Name())
		if entry.IsDir() || (ext != ".yaml" && ext != ".yml") {
			continue
		}
		spec, err := loadSpec(filepath.Join(dir, entry.Name()))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: skipping template %s: %v\n", entry.Name(), err)
			continue
		}
		specs = append(specs, spec)
	}
	return specs, nil
}

func loadSpec(path string) (Spec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Spec{}, err
	}
	var spec Spec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return Spec{}, err
	}
	if spec.Name == "" {
		spec.Name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	return spec, nil
}

func Execute(ctx context.Context, spec Spec, input string, vars map[string]string) error {
	client, err := ai.NewClient()
	if err != nil {
		return err
	}

	sysPrompt := spec.SystemPrompt
	// Apply variable substitutions
	merged := make(map[string]string)
	for k, v := range spec.Variables {
		merged[k] = v
	}
	for k, v := range vars {
		merged[k] = v
	}
	for k, v := range merged {
		sysPrompt = strings.ReplaceAll(sysPrompt, "{{"+k+"}}", v)
	}

	sysInfo := system.Detect()
	sysPrompt = sysPrompt + "\n\n" + sysInfo.ContextBlock()

	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: sysPrompt},
		{Role: openai.ChatMessageRoleUser, Content: input},
	}

	if spec.UseTools {
		runner := chatutil.NewRunner(client, sysInfo)
		text, _, err := runner.Stream(ctx, messages, chatutil.StreamOptions{
			OnContent: func(chunk string) {
				fmt.Print(chunk)
			},
		})
		if err != nil {
			return err
		}
		if text != "" {
			fmt.Println()
		}
		return nil
	}

	if spec.Stream {
		_, err := client.StreamComplete(ctx, messages, func(chunk string) {
			fmt.Print(chunk)
		})
		fmt.Println()
		return err
	}

	resp, err := client.Complete(ctx, messages)
	if err != nil {
		return err
	}
	fmt.Println(resp)
	return nil
}

func CreateStarter(name string) error {
	dir := TemplatesDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	path := filepath.Join(dir, name+".yaml")
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("template %q already exists at %s", name, path)
	}

	content := fmt.Sprintf(`name: %s
description: "A custom template"
system_prompt: |
  You are a helpful assistant.
  Respond clearly and concisely.
input_mode: args
variables:
  style: "concise"
stream: true
use_tools: false
`, name)

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return err
	}
	fmt.Printf("Created template: %s\n", path)
	return nil
}
