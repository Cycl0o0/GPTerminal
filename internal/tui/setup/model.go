package setup

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cycl0o0/GPTerminal/internal/ai"
	"github.com/cycl0o0/GPTerminal/internal/config"
)

const (
	stepWelcome = iota
	stepAPIKey
	stepBaseURL
	stepModel
	stepShell
	stepDone
)

type modelsLoadedMsg struct {
	models []string
	err    error
}

func fetchModelsCmd() tea.Cmd {
	return func() tea.Msg {
		client, err := ai.NewClient()
		if err != nil {
			return modelsLoadedMsg{err: err}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		models, err := client.ListModels(ctx)
		return modelsLoadedMsg{models: models, err: err}
	}
}

type Model struct {
	step      int
	textInput textinput.Model
	width     int
	height    int
	ready     bool

	apiKey  string
	baseURL string
	model   string
	shell   string
	err     string

	availableModels []string
	fetchingModels  bool
	modelCursor     int
	modelPickMode   bool

	savedAPIKey    bool
	savedBaseURL   bool
	savedModel     bool
	shellInstalled bool
	shellSkipped   bool
	shellError     string
}

func NewModel() Model {
	ti := textinput.New()
	ti.Focus()
	ti.CharLimit = 256
	ti.Width = 60

	shell := detectShell()

	return Model{
		step:      stepWelcome,
		textInput: ti,
		shell:     shell,
		baseURL:   config.DefaultBaseURL,
		model:     config.DefaultModel,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, tea.EnterAltScreen)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		return m, nil

	case modelsLoadedMsg:
		m.fetchingModels = false
		if msg.err == nil && len(msg.models) > 0 {
			m.availableModels = msg.models
			m.modelPickMode = true
			for i, id := range msg.models {
				if id == m.model {
					m.modelCursor = i
					break
				}
			}
		}
		return m, nil

	case tea.KeyMsg:
		m.err = ""

		// Pick mode: ↑/↓ navigation, Tab to switch to manual input
		if m.step == stepModel && m.modelPickMode {
			switch msg.Type {
			case tea.KeyCtrlC, tea.KeyEsc:
				return m, tea.Quit
			case tea.KeyUp:
				if m.modelCursor > 0 {
					m.modelCursor--
				}
				return m, nil
			case tea.KeyDown:
				if m.modelCursor < len(m.availableModels)-1 {
					m.modelCursor++
				}
				return m, nil
			case tea.KeyTab:
				m.modelPickMode = false
				m.textInput.SetValue(m.availableModels[m.modelCursor])
				m.textInput.Focus()
				return m, textinput.Blink
			case tea.KeyEnter:
				return m.handleEnter()
			}
			return m, nil
		}

		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit

		case tea.KeyTab:
			return m.handleSkip()

		case tea.KeyEnter:
			return m.handleEnter()
		}
	}

	if m.step >= stepAPIKey && m.step <= stepModel && !m.modelPickMode {
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) handleEnter() (tea.Model, tea.Cmd) {
	switch m.step {
	case stepWelcome:
		m.step = stepAPIKey
		m.textInput.SetValue("")
		m.textInput.Placeholder = "sk-... (or press Tab to skip for Ollama)"
		m.textInput.EchoMode = textinput.EchoPassword
		m.textInput.EchoCharacter = '*'
		m.textInput.Focus()
		return m, textinput.Blink

	case stepAPIKey:
		val := strings.TrimSpace(m.textInput.Value())
		if val != "" {
			if err := config.SaveAPIKey(val); err != nil {
				m.err = fmt.Sprintf("Failed to save: %v", err)
				return m, nil
			}
			m.apiKey = val
			m.savedAPIKey = true
		}
		m.step = stepBaseURL
		m.textInput.SetValue(m.baseURL)
		m.textInput.Placeholder = config.DefaultBaseURL
		m.textInput.EchoMode = textinput.EchoNormal
		m.textInput.EchoCharacter = 0
		m.textInput.Focus()
		return m, textinput.Blink

	case stepBaseURL:
		val := strings.TrimSpace(m.textInput.Value())
		if val != "" && val != config.DefaultBaseURL {
			if err := config.SaveAPIBaseURL(val); err != nil {
				m.err = fmt.Sprintf("Failed to save: %v", err)
				return m, nil
			}
			m.baseURL = val
			m.savedBaseURL = true
		}
		m.step = stepModel
		m.fetchingModels = true
		m.textInput.SetValue(m.model)
		m.textInput.Placeholder = config.DefaultModel
		m.textInput.Focus()
		return m, tea.Batch(textinput.Blink, fetchModelsCmd())

	case stepModel:
		var val string
		if m.modelPickMode && len(m.availableModels) > 0 {
			val = m.availableModels[m.modelCursor]
		} else {
			val = strings.TrimSpace(m.textInput.Value())
		}
		if val != "" && val != config.DefaultModel {
			if err := config.SaveModel(val); err != nil {
				m.err = fmt.Sprintf("Failed to save: %v", err)
				return m, nil
			}
			m.model = val
			m.savedModel = true
		}
		m.step = stepShell
		return m, nil

	case stepShell:
		if m.shell != "" {
			if err := m.autoInstallShell(); err != nil {
				m.shellError = err.Error()
			} else {
				m.shellInstalled = true
			}
		}
		m.step = stepDone
		return m, nil

	case stepDone:
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) handleSkip() (tea.Model, tea.Cmd) {
	switch m.step {
	case stepAPIKey:
		m.step = stepBaseURL
		m.textInput.SetValue(m.baseURL)
		m.textInput.Placeholder = config.DefaultBaseURL
		m.textInput.EchoMode = textinput.EchoNormal
		m.textInput.EchoCharacter = 0
		m.textInput.Focus()
		return m, textinput.Blink

	case stepBaseURL:
		m.step = stepModel
		m.fetchingModels = true
		m.textInput.SetValue(m.model)
		m.textInput.Placeholder = config.DefaultModel
		m.textInput.Focus()
		return m, tea.Batch(textinput.Blink, fetchModelsCmd())

	case stepModel:
		m.step = stepShell
		return m, nil

	case stepShell:
		m.shellSkipped = true
		m.step = stepDone
		return m, nil
	}
	return m, nil
}

func (m Model) View() string {
	if !m.ready {
		return ""
	}

	var content string
	switch m.step {
	case stepWelcome:
		content = m.viewWelcome()
	case stepAPIKey:
		content = m.viewAPIKey()
	case stepBaseURL:
		content = m.viewBaseURL()
	case stepModel:
		content = m.viewModel()
	case stepShell:
		content = m.viewShell()
	case stepDone:
		content = m.viewDone()
	}

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m Model) viewWelcome() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("GPTerminal"))
	b.WriteString("\n\n")
	b.WriteString(subtitleStyle.Render("AI-powered terminal assistant"))
	b.WriteString("\n\n")

	features := []string{
		"Command correction with 'fuck'",
		"Interactive AI chat",
		"Natural language commands",
		"Code review & editing",
		"Autonomous agent mode",
		"Speech-to-text & text-to-speech",
	}
	for _, f := range features {
		b.WriteString(fmt.Sprintf("  %s %s\n", accentStyle.Render("*"), f))
	}

	b.WriteString("\n")
	b.WriteString(hintStyle.Render("Press Enter to begin setup..."))

	return b.String()
}

func (m Model) viewAPIKey() string {
	var b strings.Builder

	b.WriteString(m.stepHeader("Step 1/4", "API Key"))
	b.WriteString("\n\n")
	b.WriteString("Enter your OpenAI API key.\n")
	b.WriteString(dimStyle.Render("If you're using Ollama or another local provider, press Tab to skip."))
	b.WriteString("\n\n")
	b.WriteString(m.textInput.View())
	b.WriteString("\n\n")

	if m.err != "" {
		b.WriteString(errorStyle.Render(m.err))
		b.WriteString("\n\n")
	}

	b.WriteString(m.navHints("Enter: save", "Tab: skip", "Esc: quit"))

	return b.String()
}

func (m Model) viewBaseURL() string {
	var b strings.Builder

	b.WriteString(m.stepHeader("Step 2/4", "API Base URL"))
	b.WriteString("\n\n")
	b.WriteString("API endpoint URL.\n")
	b.WriteString(dimStyle.Render("For Ollama use: http://localhost:11434/v1"))
	b.WriteString("\n\n")
	b.WriteString(m.textInput.View())
	b.WriteString("\n\n")

	if m.err != "" {
		b.WriteString(errorStyle.Render(m.err))
		b.WriteString("\n\n")
	}

	b.WriteString(m.navHints("Enter: save", "Tab: skip (keep default)", "Esc: quit"))

	return b.String()
}

func (m Model) viewModel() string {
	var b strings.Builder

	b.WriteString(m.stepHeader("Step 3/4", "Model"))
	b.WriteString("\n\n")

	if m.modelPickMode && len(m.availableModels) > 0 {
		b.WriteString("Select a model:\n\n")

		const windowSize = 8
		total := len(m.availableModels)
		start := m.modelCursor - windowSize/2
		if start < 0 {
			start = 0
		}
		end := start + windowSize
		if end > total {
			end = total
			start = max(0, end-windowSize)
		}

		if start > 0 {
			b.WriteString(dimStyle.Render(fmt.Sprintf("  (%d more above)\n", start)))
		}
		for i := start; i < end; i++ {
			if i == m.modelCursor {
				b.WriteString(accentStyle.Render(fmt.Sprintf("  > %s\n", m.availableModels[i])))
			} else {
				b.WriteString(fmt.Sprintf("    %s\n", m.availableModels[i]))
			}
		}
		if end < total {
			b.WriteString(dimStyle.Render(fmt.Sprintf("  (%d more below)\n", total-end)))
		}

		b.WriteString("\n")
		if m.err != "" {
			b.WriteString(errorStyle.Render(m.err))
			b.WriteString("\n\n")
		}
		b.WriteString(m.navHints("↑/↓: navigate", "Enter: select", "Tab: type manually", "Esc: quit"))
		return b.String()
	}

	if m.fetchingModels {
		b.WriteString(dimStyle.Render("Fetching available models..."))
		b.WriteString("\n\n")
	}

	b.WriteString("Which model should GPTerminal use?\n")
	b.WriteString(dimStyle.Render("Examples: gpt-4o, gpt-4o-mini, llama3, deepseek-r1"))
	b.WriteString("\n\n")
	b.WriteString(m.textInput.View())
	b.WriteString("\n\n")

	if m.err != "" {
		b.WriteString(errorStyle.Render(m.err))
		b.WriteString("\n\n")
	}

	b.WriteString(m.navHints("Enter: save", "Tab: skip (keep default)", "Esc: quit"))
	return b.String()
}

func (m Model) viewShell() string {
	var b strings.Builder

	b.WriteString(m.stepHeader("Step 4/4", "Shell Integration"))
	b.WriteString("\n\n")
	b.WriteString("Add this line to your shell config to enable aliases and shortcuts:\n\n")

	if m.shell != "" {
		rcFile := shellRCFile(m.shell)
		evalCmd := fmt.Sprintf(`eval "$(gpterminal init %s)"`, m.shell)
		if m.shell == "fish" {
			evalCmd = "gpterminal init fish | source"
		}

		b.WriteString(fmt.Sprintf("  %s  %s\n\n", labelStyle.Render("Shell:"), m.shell))
		b.WriteString(fmt.Sprintf("  %s  %s\n\n", labelStyle.Render("Add to"), rcFile+":"))
		b.WriteString(fmt.Sprintf("  %s\n", codeStyle.Render(evalCmd)))
	} else {
		b.WriteString(dimStyle.Render("  Could not detect shell. Run one of:"))
		b.WriteString("\n\n")
		b.WriteString(fmt.Sprintf("  %s\n", codeStyle.Render(`eval "$(gpterminal init bash)"`)))
		b.WriteString(fmt.Sprintf("  %s\n", codeStyle.Render(`eval "$(gpterminal init zsh)"`)))
		b.WriteString(fmt.Sprintf("  %s\n", codeStyle.Render("gpterminal init fish | source")))
	}

	b.WriteString("\n")
	b.WriteString(m.navHints("Enter: auto-install", "Tab: skip", "Esc: quit"))

	return b.String()
}

func (m Model) viewDone() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Setup Complete!"))
	b.WriteString("\n\n")

	check := successStyle.Render("[OK]")
	dash := dimStyle.Render(" --")

	if m.savedAPIKey {
		b.WriteString(fmt.Sprintf("  %s API Key     configured\n", check))
	} else {
		b.WriteString(fmt.Sprintf("  %s API Key     not set (use env OPENAI_API_KEY or gpterminal config set api_key)\n", dash))
	}

	if m.savedBaseURL {
		b.WriteString(fmt.Sprintf("  %s Base URL    %s\n", check, m.baseURL))
	} else {
		b.WriteString(fmt.Sprintf("  %s Base URL    %s (default)\n", dash, m.baseURL))
	}

	if m.savedModel {
		b.WriteString(fmt.Sprintf("  %s Model       %s\n", check, m.model))
	} else {
		b.WriteString(fmt.Sprintf("  %s Model       %s (default)\n", dash, m.model))
	}

	if m.shellInstalled {
		b.WriteString(fmt.Sprintf("  %s Shell      installed to %s\n", check, shellRCFile(m.shell)))
	} else if m.shellError != "" {
		b.WriteString(fmt.Sprintf("  %s Shell      auto-install failed: %s\n", dash, m.shellError))
	} else if m.shellSkipped {
		b.WriteString(fmt.Sprintf("  %s Shell      skipped (add manually)\n", dash))
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render(fmt.Sprintf("Config saved to: %s", config.ConfigFile())))
	b.WriteString("\n\n")
	b.WriteString("Try it out:\n")
	b.WriteString(fmt.Sprintf("  %s  Start a chat session\n", codeStyle.Render("gpterminal chat")))
	b.WriteString(fmt.Sprintf("  %s  Fix your last command\n", codeStyle.Render("gpterminal fix")))
	b.WriteString(fmt.Sprintf("  %s  Run a natural language command\n", codeStyle.Render("gpterminal run \"list large files\"")))
	b.WriteString("\n")
	b.WriteString(hintStyle.Render("Press Enter to exit."))

	return b.String()
}

func (m Model) stepHeader(step, name string) string {
	return fmt.Sprintf("%s  %s", dimStyle.Render(step), titleStyle.Render(name))
}

func (m Model) navHints(hints ...string) string {
	parts := make([]string, len(hints))
	for i, h := range hints {
		parts[i] = hintStyle.Render(h)
	}
	return strings.Join(parts, hintStyle.Render("  |  "))
}

func detectShell() string {
	// Check parent process name first — this is the shell actually running us.
	ppid := os.Getppid()
	if data, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", ppid)); err == nil {
		name := strings.TrimSpace(string(data))
		switch name {
		case "bash", "zsh", "fish":
			return name
		}
	}
	// Fall back to $SHELL (login shell).
	shell := os.Getenv("SHELL")
	if shell == "" {
		return ""
	}
	return filepath.Base(shell)
}

func (m Model) autoInstallShell() error {
	rcFile := shellRCFile(m.shell)
	if rcFile == "your shell rc file" {
		return fmt.Errorf("unsupported shell: %s", m.shell)
	}

	// Expand ~ to $HOME
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home dir: %w", err)
	}
	if strings.HasPrefix(rcFile, "~/") {
		rcFile = filepath.Join(home, rcFile[2:])
	}

	evalLine := fmt.Sprintf(`eval "$(gpterminal init %s)"`, m.shell)
	if m.shell == "fish" {
		evalLine = "gpterminal init fish | source"
	}

	// Read existing contents and check if already present.
	data, err := os.ReadFile(rcFile)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read %s: %w", rcFile, err)
	}
	if strings.Contains(string(data), evalLine) {
		return nil // already installed
	}

	// Ensure parent directory exists (for fish).
	if dir := filepath.Dir(rcFile); dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("create dir %s: %w", dir, err)
		}
	}

	f, err := os.OpenFile(rcFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open %s: %w", rcFile, err)
	}
	defer f.Close()

	// Add newline before if file exists and doesn't end with newline.
	if len(data) > 0 && data[len(data)-1] != '\n' {
		f.WriteString("\n")
	}

	_, err = f.WriteString(evalLine + "\n")
	return err
}

func shellRCFile(shell string) string {
	switch shell {
	case "bash":
		return "~/.bashrc"
	case "zsh":
		return "~/.zshrc"
	case "fish":
		return "~/.config/fish/config.fish"
	default:
		return "your shell rc file"
	}
}
