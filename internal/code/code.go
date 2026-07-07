package code

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/cycl0o0/GPTerminal/internal/ai"
	"github.com/cycl0o0/GPTerminal/internal/chatutil"
	"github.com/cycl0o0/GPTerminal/internal/config"
	"github.com/cycl0o0/GPTerminal/internal/mcp"
	"github.com/cycl0o0/GPTerminal/internal/session"
	"github.com/cycl0o0/GPTerminal/internal/system"
	"github.com/cycl0o0/GPTerminal/internal/usage"
	openai "github.com/sashabaranov/go-openai"
)

type Config struct {
	SessionName string
	Model       string // optional in-memory model override
}

func Run(ctx context.Context, cfg Config) error {
	client, err := ai.NewClient()
	if err != nil {
		return err
	}
	if cfg.Model != "" {
		config.SetActiveModel(cfg.Model)
	}

	sysInfo := system.Detect()
	cwd, _ := os.Getwd()

	var mcpReg *mcp.Registry
	if servers := config.MCPServers(); len(servers) > 0 {
		mcpReg = mcp.NewRegistry()
		if err := mcpReg.LoadFromConfig(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: MCP: %v\n", err)
		} else {
			defer mcpReg.Close()
		}
	}

	runner := chatutil.NewRunnerWithMCP(client, sysInfo, mcpReg)

	projectCtx := gatherProjectContext(cwd)

	printBanner(cwd, client.ProviderName(), cfg.SessionName)

	messages, transcript := loadCodeSession(sysInfo, projectCtx, cfg.SessionName)

	approvalReader := bufio.NewReader(os.Stdin)
	input := newInputReader()
	autoApprove := false

	const mainPrompt = "\033[1;36m>\033[0m "
	const contPrompt = "\033[90m…\033[0m "

	for {
		text, status := input.readLogicalLine(mainPrompt, contPrompt)
		switch status {
		case readEOF:
			fmt.Fprintln(os.Stderr)
			if cfg.SessionName != "" {
				saveCodeSession(cfg.SessionName, messages, transcript)
			}
			return nil
		case readCancel:
			continue
		}

		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}

		if strings.HasPrefix(text, "/") {
			quit := handleSlashCommand(slashCtx{
				text:       text,
				messages:   &messages,
				transcript: &transcript,
				sysInfo:    sysInfo,
				projectCtx: projectCtx,
				cwd:        cwd,
				session:    cfg.SessionName,
				provider:   client.ProviderName(),
				reader:     approvalReader,
			})
			if quit {
				if cfg.SessionName != "" {
					saveCodeSession(cfg.SessionName, messages, transcript)
				}
				return nil
			}
			continue
		}

		messages = append(messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: text,
		})
		transcript = append(transcript, session.ChatMessage{
			Role:      openai.ChatMessageRoleUser,
			Content:   text,
			Timestamp: time.Now().Format("15:04"),
		})

		text, finalHistory, err := runner.Stream(ctx, messages, chatutil.StreamOptions{
			AllowWriteTools: true,
			LiveContent:     true,
			OnThinking: func(t string) {
				if strings.TrimSpace(t) != "" {
					line := truncate(t, 160)
					fmt.Fprintf(os.Stderr, "\033[35m⟡ %s\033[0m\n", line)
				}
			},
			OnContent: func(chunk string) {
				fmt.Print(chunk)
			},
			OnToolCall: func(name, args string) {
				fmt.Fprintf(os.Stderr, "\n\033[33m⚡ %s\033[0m\n", name)
			},
			OnToolResult: func(name, result string) {
				fmt.Fprintf(os.Stderr, "\033[33m✓ %s\033[0m %s\n", name, firstLine(result, 140))
			},
			ApproveCommand: func(req chatutil.CommandApprovalRequest) (chatutil.ApprovalDecision, error) {
				allowAuto := req.RiskErr == nil && req.Risk != nil && req.Risk.Score <= 7
				if autoApprove && allowAuto {
					fmt.Fprintf(os.Stderr, "\033[32m✓ auto-approved: %s\033[0m\n", req.Command)
					return chatutil.ApprovalDecision{Approved: true, AutoApprove: true}, nil
				}

				fmt.Fprintf(os.Stderr, "\n\033[1mCommand:\033[0m %s\n", req.Command)
				if req.RiskErr != nil {
					fmt.Fprintf(os.Stderr, "Risk: unavailable (%v)\n", req.RiskErr)
				} else if req.Risk != nil {
					color := "\033[32m"
					if req.Risk.Score > 3 {
						color = "\033[33m"
					}
					if req.Risk.Score > 6 {
						color = "\033[31m"
					}
					fmt.Fprintf(os.Stderr, "Risk: %s%d/10 [%s]%s %s\n", color, req.Risk.Score, strings.ToUpper(req.Risk.Level), "\033[0m", req.Risk.Summary)
				}

				prompt := "Approve? [Y]es / [a]uto / [n]o: "
				if !allowAuto {
					prompt = "Approve? [Y/n]: "
				}
				fmt.Fprint(os.Stderr, prompt)
				answer, _ := approvalReader.ReadString('\n')
				answer = strings.TrimSpace(strings.ToLower(answer))

				switch answer {
				case "a", "auto":
					if allowAuto {
						autoApprove = true
						return chatutil.ApprovalDecision{Approved: true, AutoApprove: true}, nil
					}
					return chatutil.ApprovalDecision{Approved: true}, nil
				case "n", "no":
					return chatutil.ApprovalDecision{Approved: false}, nil
				default:
					return chatutil.ApprovalDecision{Approved: true}, nil
				}
			},
			ApproveFileWrite: func(req chatutil.FileWriteApprovalRequest) (chatutil.ApprovalDecision, error) {
				fmt.Fprintf(os.Stderr, "\n\033[1mFile:\033[0m %s\n", req.Path)
				fmt.Fprintf(os.Stderr, "%s\n", req.Diff)
				fmt.Fprint(os.Stderr, "Approve? [Y/n]: ")
				answer, _ := approvalReader.ReadString('\n')
				answer = strings.TrimSpace(strings.ToLower(answer))
				if answer == "n" || answer == "no" {
					return chatutil.ApprovalDecision{Approved: false}, nil
				}
				return chatutil.ApprovalDecision{Approved: true}, nil
			},
		})
		fmt.Println()

		if err != nil {
			fmt.Fprintf(os.Stderr, "\033[31mError: %v\033[0m\n", err)
			continue
		}

		messages = finalHistory
		transcript = append(transcript, session.ChatMessage{
			Role:      openai.ChatMessageRoleAssistant,
			Content:   text,
			Timestamp: time.Now().Format("15:04"),
		})

		if cfg.SessionName != "" {
			saveCodeSession(cfg.SessionName, messages, transcript)
		}
	}
}

func printBanner(cwd, provider, sessionName string) {
	model := config.Model()
	fmt.Fprintf(os.Stderr, "\033[1;36m╭───────────────────────────────────────╮\033[0m\n")
	fmt.Fprintf(os.Stderr, "\033[1;36m│   GPTCode  —  coding agent            │\033[0m\n")
	fmt.Fprintf(os.Stderr, "\033[1;36m╰───────────────────────────────────────╯\033[0m\n")
	fmt.Fprintf(os.Stderr, "\033[90mProject:  %s\033[0m\n", cwd)
	fmt.Fprintf(os.Stderr, "\033[90mModel:    %s (%s)\033[0m\n", model, provider)
	if sessionName != "" {
		fmt.Fprintf(os.Stderr, "\033[90mSession:  %s\033[0m\n", sessionName)
	}
	fmt.Fprintf(os.Stderr, "\033[90m/help for commands · Ctrl-D to exit · ↑/↓ history\033[0m\n\n")
}

type slashCtx struct {
	text       string
	messages   *[]openai.ChatCompletionMessage
	transcript *[]session.ChatMessage
	sysInfo    system.SystemInfo
	projectCtx string
	cwd        string
	session    string
	provider   string
	reader     *bufio.Reader
}

func handleSlashCommand(c slashCtx) bool {
	parts := splitCmd(c.text)
	if len(parts) == 0 {
		return false
	}
	cmd := strings.ToLower(parts[0])

	switch cmd {
	case "/quit", "/exit", "/q":
		fmt.Fprintf(os.Stderr, "\033[90mGoodbye!\033[0m\n")
		return true

	case "/help", "/h":
		fmt.Fprintf(os.Stderr, "\033[1mGPTCode Commands:\033[0m\n")
		fmt.Fprintf(os.Stderr, "  /help            Show this help\n")
		fmt.Fprintf(os.Stderr, "  /clear           Clear conversation and start fresh\n")
		fmt.Fprintf(os.Stderr, "  /compact         Summarize conversation to reduce context\n")
		fmt.Fprintf(os.Stderr, "  /model [name]    Show or switch the model for this session\n")
		fmt.Fprintf(os.Stderr, "  /tokens          Show token usage and estimated cost\n")
		fmt.Fprintf(os.Stderr, "  /add <path>      Add a file's contents to the conversation\n")
		fmt.Fprintf(os.Stderr, "  /diff            Show git diff of changes in the project\n")
		fmt.Fprintf(os.Stderr, "  /status          Show git status\n")
		fmt.Fprintf(os.Stderr, "  /revert <path>   Revert one file via git checkout (asks first)\n")
		fmt.Fprintf(os.Stderr, "  /quit            Exit GPTCode\n")

	case "/clear":
		*c.messages = []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: ai.CodeSystemPrompt(c.sysInfo.ContextBlock(), c.projectCtx)},
		}
		*c.transcript = nil
		fmt.Fprintf(os.Stderr, "\033[90mConversation cleared.\033[0m\n")

	case "/compact":
		compactConversation(c.messages, c.sysInfo, c.projectCtx)

	case "/model":
		if len(parts) < 2 {
			fmt.Fprintf(os.Stderr, "\033[90mCurrent model: %s (%s)\033[0m\n", config.Model(), c.provider)
			return false
		}
		config.SetActiveModel(parts[1])
		fmt.Fprintf(os.Stderr, "\033[32mModel set to %s for this session.\033[0m\n", config.Model())

	case "/tokens":
		u := usage.Global().CurrentUsage()
		fmt.Fprintf(os.Stderr, "\033[1mUsage (%s):\033[0m\n", u.Month)
		fmt.Fprintf(os.Stderr, "  Input tokens:  %s\n", comma(u.InputTokens))
		fmt.Fprintf(os.Stderr, "  Output tokens: %s\n", comma(u.OutputTokens))
		fmt.Fprintf(os.Stderr, "  Estimated cost: $%.4f (images $%.4f)\n", u.TotalCost, u.ImageCost)
		fmt.Fprintf(os.Stderr, "\033[90mModel: %s (%s)\033[0m\n", config.Model(), c.provider)

	case "/add":
		if len(parts) < 2 {
			fmt.Fprintf(os.Stderr, "\033[31mUsage: /add <path>\033[0m\n")
			return false
		}
		path := strings.Join(parts[1:], " ")
		full, err := resolveInside(c.cwd, path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "\033[31m%s\033[0m\n", err)
			return false
		}
		data, err := os.ReadFile(full)
		if err != nil {
			fmt.Fprintf(os.Stderr, "\033[31mError: %v\033[0m\n", err)
			return false
		}
		content := string(data)
		if len(content) > 20000 {
			content = content[:20000] + "\n...[truncated]"
		}
		rel, _ := filepath.Rel(c.cwd, full)
		*c.messages = append(*c.messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: fmt.Sprintf("Here are the full contents of %s for reference:\n\n```\n%s\n```", rel, content),
		})
		*c.transcript = append(*c.transcript, session.ChatMessage{
			Role:      openai.ChatMessageRoleUser,
			Content:   fmt.Sprintf("[added file %s]", rel),
			Timestamp: time.Now().Format("15:04"),
		})
		fmt.Fprintf(os.Stderr, "\033[32mAdded %s (%d bytes) to context.\033[0m\n", rel, len(data))

	case "/diff":
		out, err := runGitCommand(c.cwd, "diff")
		if err != nil {
			fmt.Fprintf(os.Stderr, "\033[31mError: %v\033[0m\n", err)
		} else if strings.TrimSpace(out) == "" {
			fmt.Fprintf(os.Stderr, "\033[90mNo changes.\033[0m\n")
		} else {
			fmt.Println(out)
		}

	case "/status":
		out, err := runGitCommand(c.cwd, "status", "--short")
		if err != nil {
			fmt.Fprintf(os.Stderr, "\033[31mError: %v\033[0m\n", err)
		} else if strings.TrimSpace(out) == "" {
			fmt.Fprintf(os.Stderr, "\033[90mWorking tree clean.\033[0m\n")
		} else {
			fmt.Println(out)
		}

	case "/revert":
		if len(parts) < 2 {
			fmt.Fprintf(os.Stderr, "\033[31mUsage: /revert <path>\033[0m\n")
			return false
		}
		path := strings.Join(parts[1:], " ")
		fmt.Fprintf(os.Stderr, "\033[33mRevert %s (git checkout -- %s)? [y/N]: \033[0m", path, path)
		answer, _ := c.reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" && answer != "yes" {
			fmt.Fprintf(os.Stderr, "\033[90mCancelled.\033[0m\n")
			return false
		}
		args, perr := chatSplitForGit(path)
		if perr != nil {
			fmt.Fprintf(os.Stderr, "\033[31m%s\033[0m\n", perr)
			return false
		}
		gitArgs := append([]string{"checkout", "--"}, args...)
		out, err := runGitCommand(c.cwd, gitArgs...)
		if err != nil {
			fmt.Fprintf(os.Stderr, "\033[31mError: %v\033[0m\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "\033[32mReverted %s.\033[0m\n", path)
			if strings.TrimSpace(out) != "" {
				fmt.Println(out)
			}
		}

	default:
		fmt.Fprintf(os.Stderr, "\033[31mUnknown command: %s (type /help for commands)\033[0m\n", cmd)
	}

	return false
}

func compactConversation(messages *[]openai.ChatCompletionMessage, sysInfo system.SystemInfo, projectCtx string) {
	if len(*messages) <= 2 {
		fmt.Fprintf(os.Stderr, "\033[90mNothing to compact.\033[0m\n")
		return
	}

	var summary strings.Builder
	summary.WriteString("Previous conversation summary:\n")
	count := 0
	for _, msg := range *messages {
		if msg.Role == openai.ChatMessageRoleUser {
			summary.WriteString(fmt.Sprintf("- User asked: %s\n", truncate(msg.Content, 100)))
			count++
		} else if msg.Role == openai.ChatMessageRoleAssistant && msg.Content != "" {
			summary.WriteString(fmt.Sprintf("- Assistant: %s\n", truncate(msg.Content, 100)))
			count++
		}
	}

	*messages = []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: ai.CodeSystemPrompt(sysInfo.ContextBlock(), projectCtx)},
		{Role: openai.ChatMessageRoleUser, Content: summary.String()},
		{Role: openai.ChatMessageRoleAssistant, Content: "Understood. I have context from our previous conversation. How can I help?"},
	}
	fmt.Fprintf(os.Stderr, "\033[90mCompacted %d messages into summary.\033[0m\n", count)
}

func gatherProjectContext(cwd string) string {
	var ctx strings.Builder
	ctx.WriteString("Project context:\n")
	ctx.WriteString(fmt.Sprintf("Working directory: %s\n", cwd))

	if gitRoot, err := runGitCommand(cwd, "rev-parse", "--show-toplevel"); err == nil {
		ctx.WriteString(fmt.Sprintf("Git root: %s\n", strings.TrimSpace(gitRoot)))

		if branch, err := runGitCommand(cwd, "rev-parse", "--abbrev-ref", "HEAD"); err == nil {
			ctx.WriteString(fmt.Sprintf("Branch: %s\n", strings.TrimSpace(branch)))
		}

		if status, err := runGitCommand(cwd, "status", "--short"); err == nil {
			status = strings.TrimSpace(status)
			if status == "" {
				ctx.WriteString("Git status: clean\n")
			} else {
				lines := strings.Split(status, "\n")
				if len(lines) > 20 {
					lines = append(lines[:20], fmt.Sprintf("... and %d more files", len(lines)-20))
				}
				ctx.WriteString(fmt.Sprintf("Git status:\n%s\n", strings.Join(lines, "\n")))
			}
		}
	}

	if entries, err := os.ReadDir(cwd); err == nil {
		ctx.WriteString("\nTop-level files:\n")
		count := 0
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), ".") && e.Name() != ".gitignore" {
				continue
			}
			kind := "file"
			if e.IsDir() {
				kind = "dir"
			}
			ctx.WriteString(fmt.Sprintf("  [%s] %s\n", kind, e.Name()))
			count++
			if count >= 30 {
				ctx.WriteString(fmt.Sprintf("  ... and %d more\n", len(entries)-count))
				break
			}
		}
	}

	for _, name := range []string{"go.mod", "package.json", "Cargo.toml", "pyproject.toml", "requirements.txt", "pom.xml", "Makefile", "CMakeLists.txt"} {
		path := filepath.Join(cwd, name)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			data, err := os.ReadFile(path)
			if err == nil {
				content := string(data)
				if len(content) > 2000 {
					content = content[:2000] + "\n...[truncated]"
				}
				ctx.WriteString(fmt.Sprintf("\n%s:\n%s\n", name, content))
			}
			break
		}
	}

	ctx.WriteString(loadProjectInstructions(cwd))

	return ctx.String()
}

// loadProjectInstructions reads the first present agent-instruction file from
// the project root (AGENTS.md, CLAUDE.md, GPTERMINAL.md, .cursorrules,
// .windsurfrules) and returns it as a bounded context block.
func loadProjectInstructions(cwd string) string {
	for _, name := range []string{"AGENTS.md", "CLAUDE.md", "GPTERMINAL.md", ".cursorrules", ".windsurfrules"} {
		path := filepath.Join(cwd, name)
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		content := strings.TrimSpace(string(data))
		if content == "" {
			continue
		}
		if len(content) > 4000 {
			content = content[:4000] + "\n...[truncated]"
		}
		return fmt.Sprintf("\nProject instructions (%s):\n%s\n", name, content)
	}
	return ""
}

func loadCodeSession(sysInfo system.SystemInfo, projectCtx, sessionName string) ([]openai.ChatCompletionMessage, []session.ChatMessage) {
	baseHistory := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: ai.CodeSystemPrompt(sysInfo.ContextBlock(), projectCtx)},
	}
	if sessionName == "" {
		return baseHistory, nil
	}

	record, err := session.Load(sessionName)
	if err != nil || record.Kind != session.KindCode || record.Chat == nil {
		return baseHistory, nil
	}

	history := record.Chat.History
	if len(history) == 0 {
		history = baseHistory
	}
	transcript := make([]session.ChatMessage, len(record.Chat.Transcript))
	copy(transcript, record.Chat.Transcript)

	fmt.Fprintf(os.Stderr, "\033[90mResumed session: %s (%d messages)\033[0m\n", sessionName, len(transcript))
	return history, transcript
}

func saveCodeSession(name string, messages []openai.ChatCompletionMessage, transcript []session.ChatMessage) {
	_ = session.Save(&session.Record{
		Kind: session.KindCode,
		Name: name,
		Chat: &session.ChatData{
			Transcript: transcript,
			History:    messages,
		},
	})
}

func runGitCommand(cwd string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = cwd
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func truncate(s string, max int) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	if len(s) > max {
		return s[:max] + "..."
	}
	return s
}

func firstLine(s string, max int) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if len(s) > max {
		s = s[:max] + "..."
	}
	if s == "" {
		return "(no output)"
	}
	return s
}

func comma(n int) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	var b strings.Builder
	pre := len(s) % 3
	if pre > 0 {
		b.WriteString(s[:pre])
		if len(s) > pre {
			b.WriteByte(',')
		}
	}
	for i := pre; i < len(s); i += 3 {
		b.WriteString(s[i : i+3])
		if i+3 < len(s) {
			b.WriteByte(',')
		}
	}
	return b.String()
}

// splitCmd splits a slash-command line into tokens, honoring quoted args.
func splitCmd(line string) []string {
	parts, err := splitQuoted(strings.TrimSpace(line))
	if err != nil || len(parts) == 0 {
		return strings.Fields(line)
	}
	return parts
}

// chatSplitForGit splits a path arg honoring quotes, for /revert.
func chatSplitForGit(s string) ([]string, error) {
	return splitQuoted(s)
}

// splitQuoted is a shell-like splitter that honors single/double quotes and
// backslash escapes (no operator rejection — used for slash-command args).
func splitQuoted(s string) ([]string, error) {
	var args []string
	var cur strings.Builder
	haveTok := false
	inSingle, inDouble := false, false
	flush := func() {
		if haveTok {
			args = append(args, cur.String())
			cur.Reset()
			haveTok = false
		}
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case inSingle:
			if c == '\'' {
				inSingle = false
			} else {
				cur.WriteByte(c)
			}
		case inDouble:
			if c == '"' {
				inDouble = false
			} else if c == '\\' && i+1 < len(s) {
				cur.WriteByte(s[i+1])
				i++
			} else {
				cur.WriteByte(c)
			}
		case c == '\'':
			inSingle = true
			haveTok = true
		case c == '"':
			inDouble = true
			haveTok = true
		case c == '\\':
			if i+1 < len(s) {
				cur.WriteByte(s[i+1])
				i++
				haveTok = true
			}
		case c == ' ' || c == '\t':
			flush()
		default:
			cur.WriteByte(c)
			haveTok = true
		}
	}
	if inSingle || inDouble {
		return nil, fmt.Errorf("unterminated quote")
	}
	flush()
	return args, nil
}

// resolveInside resolves p under cwd, rejecting escapes.
func resolveInside(cwd, p string) (string, error) {
	cleaned := filepath.Clean(p)
	if filepath.IsAbs(cleaned) {
		return "", fmt.Errorf("absolute paths are not allowed")
	}
	full := filepath.Join(cwd, cleaned)
	rel, err := filepath.Rel(cwd, full)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path %q escapes the project directory", p)
	}
	return full, nil
}
