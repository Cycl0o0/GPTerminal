package aishell

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/cycl0o0/GPTerminal/internal/ai"
	"github.com/cycl0o0/GPTerminal/internal/risk"
	"github.com/cycl0o0/GPTerminal/internal/system"
	openai "github.com/sashabaranov/go-openai"
)

func Run(ctx context.Context) error {
	client, err := ai.NewClient()
	if err != nil {
		return err
	}

	sysInfo := system.Detect()
	cwd, _ := os.Getwd()
	shell := detectShell()

	printBanner(cwd, shell)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT)
	go func() {
		for range sigCh {
			fmt.Fprintln(os.Stderr)
		}
	}()

	reader := bufio.NewReader(os.Stdin)
	var history []commandRecord

	for {
		prompt := formatPrompt(cwd)
		fmt.Fprint(os.Stderr, prompt)

		input, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		if handleBuiltin(input, &cwd) {
			continue
		}

		if strings.HasPrefix(input, "?") {
			query := strings.TrimSpace(strings.TrimPrefix(input, "?"))
			if query == "" {
				fmt.Fprintln(os.Stderr, "\033[90mUsage: ? <question or description>\033[0m")
				continue
			}
			handleAIQuery(ctx, client, sysInfo, query, cwd, history)
			continue
		}

		if strings.HasPrefix(input, "!") {
			query := strings.TrimSpace(strings.TrimPrefix(input, "!"))
			if query == "" {
				fmt.Fprintln(os.Stderr, "\033[90mUsage: ! <description of what you want to do>\033[0m")
				continue
			}
			generated := handleVibeGenerate(ctx, client, sysInfo, query, cwd)
			if generated != "" {
				exitCode, output := executeInShell(generated, cwd, shell)
				history = appendHistory(history, generated, exitCode, output)
				if exitCode != 0 {
					offerFix(ctx, client, sysInfo, generated, output, cwd, shell, reader, &history)
				}
			}
			continue
		}

		exitCode, output := executeInShell(input, cwd, shell)
		history = appendHistory(history, input, exitCode, output)

		if exitCode != 0 {
			offerFix(ctx, client, sysInfo, input, output, cwd, shell, reader, &history)
		}
	}

	return nil
}

type commandRecord struct {
	Command  string
	ExitCode int
	Output   string
}

func printBanner(cwd, shell string) {
	fmt.Fprintf(os.Stderr, "\033[1;35m╭─────────────────────────────────────╮\033[0m\n")
	fmt.Fprintf(os.Stderr, "\033[1;35m│          GPTerminal Shell           │\033[0m\n")
	fmt.Fprintf(os.Stderr, "\033[1;35m│       AI-Enhanced Terminal          │\033[0m\n")
	fmt.Fprintf(os.Stderr, "\033[1;35m╰─────────────────────────────────────╯\033[0m\n")
	fmt.Fprintf(os.Stderr, "\033[90mShell: %s | Dir: %s\033[0m\n", shell, cwd)
	fmt.Fprintf(os.Stderr, "\033[90mTips: ? ask AI | ! generate command | exit to quit\033[0m\n\n")
}

func formatPrompt(cwd string) string {
	home, _ := os.UserHomeDir()
	display := cwd
	if home != "" && strings.HasPrefix(cwd, home) {
		display = "~" + cwd[len(home):]
	}

	branch := gitBranch(cwd)
	if branch != "" {
		return fmt.Sprintf("\033[1;35m%s\033[0m \033[36m(%s)\033[0m\033[1;35m ❯\033[0m ", display, branch)
	}
	return fmt.Sprintf("\033[1;35m%s ❯\033[0m ", display)
}

func handleBuiltin(input string, cwd *string) bool {
	switch {
	case input == "exit" || input == "quit":
		fmt.Fprintln(os.Stderr, "\033[90mGoodbye!\033[0m")
		os.Exit(0)
		return true

	case input == "help":
		fmt.Fprintf(os.Stderr, "\033[1mGPTerminal Shell Commands:\033[0m\n")
		fmt.Fprintf(os.Stderr, "  ? <question>       Ask AI a question about your project or system\n")
		fmt.Fprintf(os.Stderr, "  ! <description>    Generate and run a command from natural language\n")
		fmt.Fprintf(os.Stderr, "  cd <dir>           Change directory\n")
		fmt.Fprintf(os.Stderr, "  exit               Exit the shell\n")
		fmt.Fprintf(os.Stderr, "\033[90mAll other input runs as a normal shell command.\033[0m\n")
		fmt.Fprintf(os.Stderr, "\033[90mFailed commands trigger automatic AI fix suggestions.\033[0m\n")
		return true

	case strings.HasPrefix(input, "cd ") || input == "cd":
		dir := strings.TrimSpace(strings.TrimPrefix(input, "cd"))
		if dir == "" || dir == "~" {
			dir, _ = os.UserHomeDir()
		} else if strings.HasPrefix(dir, "~/") {
			home, _ := os.UserHomeDir()
			dir = filepath.Join(home, dir[2:])
		} else if !filepath.IsAbs(dir) {
			dir = filepath.Join(*cwd, dir)
		}

		resolved, err := filepath.Abs(dir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "\033[31mcd: %v\033[0m\n", err)
			return true
		}
		if info, err := os.Stat(resolved); err != nil {
			fmt.Fprintf(os.Stderr, "\033[31mcd: %v\033[0m\n", err)
		} else if !info.IsDir() {
			fmt.Fprintf(os.Stderr, "\033[31mcd: not a directory: %s\033[0m\n", resolved)
		} else {
			*cwd = resolved
			os.Chdir(resolved)
		}
		return true

	default:
		return false
	}
}

func executeInShell(command, cwd, shell string) (int, string) {
	cmd := exec.Command(shell, "-c", command)
	cmd.Dir = cwd
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err == nil {
		return 0, ""
	}

	exitCode := 1
	if exitErr, ok := err.(*exec.ExitError); ok {
		exitCode = exitErr.ExitCode()
	}

	cmd2 := exec.Command(shell, "-c", command)
	cmd2.Dir = cwd
	out, _ := cmd2.CombinedOutput()
	output := strings.TrimSpace(string(out))
	if len(output) > 500 {
		output = output[:500] + "..."
	}

	return exitCode, output
}

func offerFix(ctx context.Context, client *ai.Client, sysInfo system.SystemInfo, command, output, cwd, shell string, reader *bufio.Reader, history *[]commandRecord) {
	fmt.Fprintf(os.Stderr, "\033[33m⚡ AI fix available. Apply? [Y/n/i(gnore)]: \033[0m")
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))

	if answer == "n" || answer == "no" || answer == "i" || answer == "ignore" {
		return
	}

	userMsg := fmt.Sprintf("Failed command: %s\nExit code: non-zero\nWorking directory: %s", command, cwd)
	if output != "" {
		userMsg += fmt.Sprintf("\n\nError output:\n%s", output)
	}

	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: ai.FixSystemPrompt(sysInfo.ContextBlock())},
		{Role: openai.ChatMessageRoleUser, Content: userMsg},
	}

	fmt.Fprint(os.Stderr, "\033[90mThinking...\033[0m")
	resp, err := client.Complete(ctx, messages)
	fmt.Fprint(os.Stderr, "\r            \r")
	if err != nil {
		fmt.Fprintf(os.Stderr, "\033[31mAI error: %v\033[0m\n", err)
		return
	}

	suggestion := strings.TrimSpace(resp)
	if suggestion == "UNFIXABLE" {
		fmt.Fprintln(os.Stderr, "\033[90mCould not determine a fix.\033[0m")
		return
	}

	fmt.Fprintf(os.Stderr, "\033[32mFix: %s\033[0m\n", suggestion)
	fmt.Fprintf(os.Stderr, "Run fix? [Y/n]: ")
	answer2, _ := reader.ReadString('\n')
	answer2 = strings.TrimSpace(strings.ToLower(answer2))

	if answer2 == "n" || answer2 == "no" {
		return
	}

	exitCode, fixOutput := executeInShell(suggestion, cwd, shell)
	*history = appendHistory(*history, suggestion, exitCode, fixOutput)
}

func handleAIQuery(ctx context.Context, client *ai.Client, sysInfo system.SystemInfo, query, cwd string, history []commandRecord) {
	contextInfo := fmt.Sprintf("Working directory: %s\n", cwd)
	if len(history) > 0 {
		contextInfo += "Recent commands:\n"
		start := 0
		if len(history) > 5 {
			start = len(history) - 5
		}
		for _, h := range history[start:] {
			contextInfo += fmt.Sprintf("  $ %s (exit: %d)\n", h.Command, h.ExitCode)
		}
	}

	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: ai.ShellQuerySystemPrompt(sysInfo.ContextBlock())},
		{Role: openai.ChatMessageRoleUser, Content: fmt.Sprintf("%s\n\n%s", contextInfo, query)},
	}

	fmt.Fprint(os.Stderr, "\033[90mThinking...\033[0m")
	resp, err := client.Complete(ctx, messages)
	fmt.Fprint(os.Stderr, "\r            \r")
	if err != nil {
		fmt.Fprintf(os.Stderr, "\033[31mAI error: %v\033[0m\n", err)
		return
	}

	fmt.Println(resp)
}

func handleVibeGenerate(ctx context.Context, client *ai.Client, sysInfo system.SystemInfo, query, cwd string) string {
	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: ai.VibeSystemPrompt(sysInfo.ContextBlock())},
		{Role: openai.ChatMessageRoleUser, Content: fmt.Sprintf("Working directory: %s\n\n%s", cwd, query)},
	}

	fmt.Fprint(os.Stderr, "\033[90mGenerating...\033[0m")
	resp, err := client.Complete(ctx, messages)
	fmt.Fprint(os.Stderr, "\r               \r")
	if err != nil {
		fmt.Fprintf(os.Stderr, "\033[31mAI error: %v\033[0m\n", err)
		return ""
	}

	command := strings.TrimSpace(resp)
	fmt.Fprintf(os.Stderr, "\033[1mCommand:\033[0m %s\n", command)

	rr, _ := risk.Evaluate(ctx, command)
	if rr != nil {
		color := "\033[32m"
		if rr.Score > 3 {
			color = "\033[33m"
		}
		if rr.Score > 6 {
			color = "\033[31m"
		}
		fmt.Fprintf(os.Stderr, "Risk: %s%d/10 [%s]\033[0m %s\n", color, rr.Score, strings.ToUpper(rr.Level), rr.Summary)
	}

	fmt.Fprint(os.Stderr, "[Y]es / [n]o / [e]dit: ")
	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))

	switch answer {
	case "n", "no":
		return ""
	case "e", "edit":
		fmt.Fprint(os.Stderr, "Edit command: ")
		edited, _ := reader.ReadString('\n')
		edited = strings.TrimSpace(edited)
		if edited == "" {
			return ""
		}
		return edited
	default:
		return command
	}
}

func gitBranch(cwd string) string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func detectShell() string {
	if s := os.Getenv("SHELL"); s != "" {
		return s
	}
	for _, name := range []string{"bash", "sh"} {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	return "sh"
}

func appendHistory(history []commandRecord, cmd string, exitCode int, output string) []commandRecord {
	history = append(history, commandRecord{
		Command:  cmd,
		ExitCode: exitCode,
		Output:   output,
	})
	if len(history) > 50 {
		history = history[len(history)-50:]
	}
	return history
}
