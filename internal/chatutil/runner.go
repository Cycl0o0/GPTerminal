package chatutil

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/cycl0o0/GPTerminal/internal/ai"
	"github.com/cycl0o0/GPTerminal/internal/config"
	"github.com/cycl0o0/GPTerminal/internal/diffutil"
	"github.com/cycl0o0/GPTerminal/internal/fileutil"
	"github.com/cycl0o0/GPTerminal/internal/hooks"
	"github.com/cycl0o0/GPTerminal/internal/mcp"
	"github.com/cycl0o0/GPTerminal/internal/memory"
	"github.com/cycl0o0/GPTerminal/internal/risk"
	"github.com/cycl0o0/GPTerminal/internal/system"
	openai "github.com/sashabaranov/go-openai"
)

const (
	maxToolRounds      = 50
	maxToolOutputChars = 12000
	maxReadFileChars   = 100000
	maxListEntries     = 200
	maxGlobResults     = 200
	maxStdinBytes      = 100000
)

type CommandApprovalRequest struct {
	Command string
	Risk    *risk.RiskResult
	RiskErr error
}

type FileWriteApprovalRequest struct {
	Path     string
	Diff     string
	Existing bool
}

type ApprovalDecision struct {
	Approved    bool
	AutoApprove bool
}

type StreamOptions struct {
	AllowWriteTools bool
	// LiveContent streams assistant text chunk-by-chunk via OnContent as it
	// arrives, including narration that precedes tool calls. When false (the
	// default for chat/agent), content is emitted once after the turn and only
	// when no tool calls were produced.
	LiveContent      bool
	OnThinking       func(string)
	OnContent        func(string)
	OnToolCall       func(name, arguments string)
	OnToolResult     func(name, result string)
	ApproveCommand   func(CommandApprovalRequest) (ApprovalDecision, error)
	ApproveFileWrite func(FileWriteApprovalRequest) (ApprovalDecision, error)
}

type Runner struct {
	client  *ai.Client
	workDir string
	mcp     *mcp.Registry
	hooks   *hooks.Registry
}

func NewRunner(client *ai.Client, sysInfo system.SystemInfo) *Runner {
	workDir, err := os.Getwd()
	if err != nil {
		workDir = sysInfo.WorkDir
	}
	if abs, err := filepath.Abs(workDir); err == nil {
		workDir = abs
	}
	return &Runner{
		client:  client,
		workDir: workDir,
	}
}

func NewRunnerWithMCP(client *ai.Client, sysInfo system.SystemInfo, registry *mcp.Registry) *Runner {
	r := NewRunner(client, sysInfo)
	r.mcp = registry
	return r
}

func (r *Runner) SetMCP(registry *mcp.Registry) {
	r.mcp = registry
}

func (r *Runner) SetHooks(registry *hooks.Registry) {
	r.hooks = registry
}

func (r *Runner) Complete(ctx context.Context, history []openai.ChatCompletionMessage) (string, []openai.ChatCompletionMessage, error) {
	return r.Stream(ctx, history, StreamOptions{})
}

func (r *Runner) Stream(ctx context.Context, history []openai.ChatCompletionMessage, opts StreamOptions) (string, []openai.ChatCompletionMessage, error) {
	messages := cloneMessages(history)
	isOpenClaw := r.client.IsOpenClaw()

	for i := 0; i < maxToolRounds; i++ {
		msg, err := r.streamAssistant(ctx, messages, opts)
		if err != nil {
			return "", messages, err
		}
		messages = append(messages, msg)

		if len(msg.ToolCalls) == 0 || isOpenClaw {
			return strings.TrimSpace(msg.Content), messages, nil
		}

		for _, call := range msg.ToolCalls {
			if opts.OnToolCall != nil {
				opts.OnToolCall(call.Function.Name, call.Function.Arguments)
			}

			output := r.executeToolCall(ctx, call, opts)
			if opts.OnToolResult != nil {
				opts.OnToolResult(call.Function.Name, output)
			}

			messages = append(messages, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				Name:       call.Function.Name,
				ToolCallID: call.ID,
				Content:    output,
			})
		}
	}

	return "", messages, fmt.Errorf("chat tool loop exceeded %d rounds", maxToolRounds)
}

func (r *Runner) streamAssistant(ctx context.Context, history []openai.ChatCompletionMessage, opts StreamOptions) (openai.ChatCompletionMessage, error) {
	isOpenClaw := r.client.IsOpenClaw()

	var tools []openai.Tool
	if !isOpenClaw {
		tools = chatTools(opts.AllowWriteTools)
		if r.mcp != nil {
			tools = append(tools, r.mcp.Tools()...)
		}
	}

	req := openai.ChatCompletionRequest{
		Model:             config.Model(),
		Messages:          history,
		Temperature:       config.Temperature(),
		MaxTokens:         config.MaxTokens(),
		Tools:             tools,
		ParallelToolCalls: false,
		StreamOptions: &openai.StreamOptions{
			IncludeUsage: true,
		},
	}

	stream, err := r.createStreamWithRetry(ctx, req)
	if err != nil {
		return openai.ChatCompletionMessage{}, err
	}
	defer stream.Close()

	var content strings.Builder
	var reasoning strings.Builder
	toolCalls := map[int]*openai.ToolCall{}
	var usageData *openai.Usage

	for {
		evt, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return openai.ChatCompletionMessage{}, fmt.Errorf("stream recv error: %w", err)
		}
		if evt.Usage != nil {
			usageData = evt.Usage
		}
		if evt.ReasoningContent != "" {
			reasoning.WriteString(evt.ReasoningContent)
			if isOpenClaw && opts.OnThinking != nil {
				opts.OnThinking(evt.ReasoningContent)
			}
		}
		if evt.Content != "" {
			content.WriteString(evt.Content)
			// Stream text live: OpenClaw forwards its events inline, and any
			// caller that opted into LiveContent wants chunk-by-chunk text
			// (including narration that precedes tool calls).
			if (isOpenClaw || opts.LiveContent) && opts.OnContent != nil {
				opts.OnContent(evt.Content)
			}
		}
		if evt.ServerToolCall != nil && opts.OnToolCall != nil {
			opts.OnToolCall(evt.ServerToolCall.Name, evt.ServerToolCall.Arguments)
		}
		if evt.ServerToolResult != nil && opts.OnToolResult != nil {
			opts.OnToolResult(evt.ServerToolResult.Name, evt.ServerToolResult.Result)
		}
		if !isOpenClaw {
			mergeToolCalls(toolCalls, evt.ToolCalls)
		}
	}

	ordered := orderedToolCalls(toolCalls)

	// Reasoning and (non-live) content are emitted once, only on the final,
	// tool-call-free turn. OpenClaw already streamed both live inline above,
	// so it is excluded here to avoid duplication; LiveContent callers already
	// received their text chunk-by-chunk too.
	if !isOpenClaw && len(ordered) == 0 {
		if reasoning.Len() > 0 && opts.OnThinking != nil {
			opts.OnThinking(strings.TrimSpace(reasoning.String()))
		}
		if !opts.LiveContent && content.Len() > 0 && opts.OnContent != nil {
			opts.OnContent(content.String())
		}
	}

	r.client.RecordUsage(config.Model(), usageData)
	return openai.ChatCompletionMessage{
		Role:      openai.ChatMessageRoleAssistant,
		Content:   content.String(),
		ToolCalls: ordered,
	}, nil
}

// createStreamWithRetry opens a chat-completion stream, retrying transient
// (rate-limit / server / network) failures a small number of times with
// backoff. Only stream creation is retried; errors that surface mid-stream
// (during Recv) are returned immediately.
func (r *Runner) createStreamWithRetry(ctx context.Context, req openai.ChatCompletionRequest) (ai.ChatStream, error) {
	const maxAttempts = 3
	backoffs := []time.Duration{500 * time.Millisecond, time.Second, 2 * time.Second}

	var stream ai.ChatStream
	var err error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		stream, err = r.client.CreateChatCompletionStream(ctx, req)
		if err == nil {
			return stream, nil
		}
		if !isRetryableStreamErr(err) {
			return nil, err
		}
		if attempt+1 < maxAttempts {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoffs[attempt]):
			}
		}
	}
	return nil, err
}

// isRetryableStreamErr reports whether a stream-creation error is worth
// retrying. It walks the error chain (client errors are wrapped in
// *gperr.APIError) to find the underlying provider/net error.
func isRetryableStreamErr(err error) bool {
	if err == nil {
		return false
	}
	var apiErr *openai.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.HTTPStatusCode {
		case 408, 409, 429, 500, 502, 503, 504:
			return true
		}
		return false
	}
	// Network-level failures (DNS, connection reset, timeout, EOF).
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	return false
}

func IsStdoutTTY() bool {
	info, err := os.Stdout.Stat()
	if err != nil {
		return true
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func HasPipedStdin(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice == 0
}

func ReadPipedStdin(r io.Reader) (string, error) {
	limited := io.LimitReader(r, maxStdinBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return "", fmt.Errorf("read stdin: %w", err)
	}

	truncated := len(data) > maxStdinBytes
	if truncated {
		data = data[:maxStdinBytes]
	}

	content := strings.TrimSpace(string(data))
	if truncated {
		content += "\n...[stdin truncated]"
	}
	return content, nil
}

func BuildUserMessage(prompt, stdin string) string {
	prompt = strings.TrimSpace(prompt)
	stdin = strings.TrimSpace(stdin)

	switch {
	case prompt != "" && stdin != "":
		return fmt.Sprintf("%s\n\nPiped stdin:\n```text\n%s\n```", prompt, stdin)
	case stdin != "":
		return fmt.Sprintf("Analyze this piped stdin input:\n\n```text\n%s\n```", stdin)
	default:
		return prompt
	}
}

func cloneMessages(src []openai.ChatCompletionMessage) []openai.ChatCompletionMessage {
	dst := make([]openai.ChatCompletionMessage, len(src))
	copy(dst, src)
	return dst
}

func chatTools(allowWrite bool) []openai.Tool {
	tools := []openai.Tool{
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "read_file",
				Description: "Read a local text file from the current working directory. Use this for source files, logs, configs, markdown, and other text files. For large files, use offset and limit (1-indexed lines) to page through specific sections instead of reading the whole file at once.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"path": map[string]any{
							"type":        "string",
							"description": "Path to the file, relative to the current working directory.",
						},
						"offset": map[string]any{
							"type":        "integer",
							"description": "Optional 1-indexed line number to start reading from. Defaults to the first line.",
						},
						"limit": map[string]any{
							"type":        "integer",
							"description": "Optional maximum number of lines to read starting at offset.",
						},
					},
					"required": []string{"path"},
				},
			},
		},
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "list_directory",
				Description: "List files and directories inside the current working directory. Use this to explore the workspace structure.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"path": map[string]any{
							"type":        "string",
							"description": "Directory path to list, relative to the current working directory. Defaults to the current working directory when omitted.",
						},
					},
				},
			},
		},
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "search_text",
				Description: "Search for text in files inside the current working directory using ripgrep-style matching. Use this to find code, symbols, or config values.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"pattern": map[string]any{
							"type":        "string",
							"description": "Text or regex pattern to search for.",
						},
						"path": map[string]any{
							"type":        "string",
							"description": "Path to search inside, relative to the current working directory. Defaults to the current working directory when omitted.",
						},
					},
					"required": []string{"pattern"},
				},
			},
		},
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "glob",
				Description: "Find files by name pattern, fast. Use this to discover files (e.g. all Go files, all configs) before reading them. Supports * and ** wildcards and brace expansion, e.g. \"**/*.go\", \"src/**/*.{ts,tsx}\".",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"pattern": map[string]any{
							"type":        "string",
							"description": "Glob pattern, relative to the current working directory. Supports **, *, ?, and {a,b} groups.",
						},
						"path": map[string]any{
							"type":        "string",
							"description": "Directory to search in, relative to the current working directory. Defaults to the current working directory when omitted.",
						},
					},
					"required": []string{"pattern"},
				},
			},
		},
	}

	tools = append(tools, openai.Tool{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        "web_search",
			Description: "Search the web using DuckDuckGo and return the top results. Use this to find current information, documentation, or answers not available locally.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "The search query.",
					},
				},
				"required": []string{"query"},
			},
		},
	}, openai.Tool{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        "fetch_url",
			Description: "Fetch a web page or API endpoint and return its text content. Use this to read documentation, articles, or API responses from a known URL.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"url": map[string]any{
						"type":        "string",
						"description": "The URL to fetch (http or https).",
					},
				},
				"required": []string{"url"},
			},
		},
	})

	tools = append(tools, openai.Tool{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        "save_memory",
			Description: "Save a fact or preference to persistent memory so it can be recalled in future conversations. Use this to remember the user's project context, preferences, tech stack, or any important detail they mention.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"key": map[string]any{
						"type":        "string",
						"description": "A short descriptive label for this memory (e.g. \"preferred_language\", \"project_goal\", \"user_name\").",
					},
					"value": map[string]any{
						"type":        "string",
						"description": "The fact or detail to remember.",
					},
				},
				"required": []string{"key", "value"},
			},
		},
	}, openai.Tool{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        "delete_memory",
			Description: "Delete a previously saved memory by key.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"key": map[string]any{
						"type":        "string",
						"description": "The key of the memory to delete.",
					},
				},
				"required": []string{"key"},
			},
		},
	})

	runDescription := "Run a safe, read-only terminal command for inspection from the current working directory. No shell operators are allowed. Prefer simple commands like git status, git diff, rg, ls, find, cat, pwd, uname, ps, and whoami."
	if allowWrite {
		runDescription = "Run a terminal command inside the current working directory. No shell operators are allowed. Read-only commands run directly. Commands that modify files or run project tasks require user approval before execution."
	}
	tools = append(tools, openai.Tool{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        "run_command",
			Description: runDescription,
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"command": map[string]any{
						"type":        "string",
						"description": "A simple command without shell operators.",
					},
				},
				"required": []string{"command"},
			},
		},
	})

	tools = append(tools, openai.Tool{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        "execute_code",
			Description: "Write and execute a code snippet locally. Supported languages: python, javascript, bash, ruby, go. Returns stdout, stderr, and exit code. User must approve before execution.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"language": map[string]any{
						"type":        "string",
						"description": "One of: python, javascript, bash, ruby, go.",
					},
					"code": map[string]any{
						"type":        "string",
						"description": "The source code to execute.",
					},
				},
				"required": []string{"language", "code"},
			},
		},
	})

	if allowWrite {
		tools = append(tools, openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "write_file",
				Description: "Write or overwrite a text file inside the current working directory. The user will see a diff and must approve the write before it is applied. Prefer edit_file for targeted changes to existing files.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"path": map[string]any{
							"type":        "string",
							"description": "File path relative to the current working directory.",
						},
						"content": map[string]any{
							"type":        "string",
							"description": "The full file contents to write.",
						},
					},
					"required": []string{"path", "content"},
				},
			},
		}, openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "edit_file",
				Description: "Apply targeted edits to an existing file using search-and-replace operations. Each operation finds an exact match of old_text and replaces it with new_text. Safer and more efficient than write_file for modifying existing files.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"path": map[string]any{
							"type":        "string",
							"description": "File path relative to the current working directory.",
						},
						"edits": map[string]any{
							"type":        "array",
							"description": "List of search-and-replace operations to apply in order.",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"old_text": map[string]any{
										"type":        "string",
										"description": "Exact text to find in the file. Must match uniquely.",
									},
									"new_text": map[string]any{
										"type":        "string",
										"description": "Replacement text.",
									},
								},
								"required": []string{"old_text", "new_text"},
							},
						},
					},
					"required": []string{"path", "edits"},
				},
			},
		})
	}

	return tools
}

func (r *Runner) executeToolCall(ctx context.Context, call openai.ToolCall, opts StreamOptions) string {
	switch call.Function.Name {
	case "read_file":
		var args struct {
			Path   string `json:"path"`
			Offset int    `json:"offset"`
			Limit  int    `json:"limit"`
		}
		if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
			return "Error: invalid tool arguments: " + err.Error()
		}
		out, err := r.readFile(args.Path, args.Offset, args.Limit)
		if err != nil {
			return "Error: " + err.Error()
		}
		return out

	case "list_directory":
		var args struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
			return "Error: invalid tool arguments: " + err.Error()
		}
		out, err := r.listDirectory(args.Path)
		if err != nil {
			return "Error: " + err.Error()
		}
		return out

	case "search_text":
		var args struct {
			Pattern string `json:"pattern"`
			Path    string `json:"path"`
		}
		if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
			return "Error: invalid tool arguments: " + err.Error()
		}
		out, err := r.searchText(args.Pattern, args.Path)
		if err != nil {
			return "Error: " + err.Error()
		}
		return out

	case "glob":
		var args struct {
			Pattern string `json:"pattern"`
			Path    string `json:"path"`
		}
		if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
			return "Error: invalid tool arguments: " + err.Error()
		}
		out, err := r.globFiles(args.Pattern, args.Path)
		if err != nil {
			return "Error: " + err.Error()
		}
		return out

	case "run_command":
		var args struct {
			Command string `json:"command"`
		}
		if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
			return "Error: invalid tool arguments: " + err.Error()
		}
		out, err := r.runCommand(ctx, args.Command, opts)
		if err != nil {
			return "Error: " + err.Error()
		}
		return out

	case "execute_code":
		var args struct {
			Language string `json:"language"`
			Code     string `json:"code"`
		}
		if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
			return "Error: invalid tool arguments: " + err.Error()
		}
		out, err := r.executeCode(ctx, args.Language, args.Code, opts)
		if err != nil {
			return "Error: " + err.Error()
		}
		return out

	case "write_file":
		var args struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
			return "Error: invalid tool arguments: " + err.Error()
		}
		out, err := r.writeFile(args.Path, args.Content, opts)
		if err != nil {
			return "Error: " + err.Error()
		}
		return out

	case "edit_file":
		var args struct {
			Path  string   `json:"path"`
			Edits []editOp `json:"edits"`
		}
		if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
			return "Error: invalid tool arguments: " + err.Error()
		}
		out, err := r.editFile(args.Path, args.Edits, opts)
		if err != nil {
			return "Error: " + err.Error()
		}
		return out

	case "save_memory":
		var args struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		}
		if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
			return "Error: invalid tool arguments: " + err.Error()
		}
		store, err := memory.Load()
		if err != nil {
			return "Error: " + err.Error()
		}
		store.Set(args.Key, args.Value)
		if err := store.Save(); err != nil {
			return "Error: " + err.Error()
		}
		return fmt.Sprintf("Saved memory: %s = %s", args.Key, args.Value)

	case "delete_memory":
		var args struct {
			Key string `json:"key"`
		}
		if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
			return "Error: invalid tool arguments: " + err.Error()
		}
		store, err := memory.Load()
		if err != nil {
			return "Error: " + err.Error()
		}
		if store.Delete(args.Key) {
			if err := store.Save(); err != nil {
				return "Error: " + err.Error()
			}
			return fmt.Sprintf("Deleted memory: %s", args.Key)
		}
		return fmt.Sprintf("Memory not found: %s", args.Key)

	case "web_search":
		var args struct {
			Query string `json:"query"`
		}
		if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
			return "Error: invalid tool arguments: " + err.Error()
		}
		out, err := webSearch(ctx, args.Query)
		if err != nil {
			return "Error: " + err.Error()
		}
		return truncateText(out, maxToolOutputChars)

	case "fetch_url":
		var args struct {
			URL string `json:"url"`
		}
		if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
			return "Error: invalid tool arguments: " + err.Error()
		}
		out, err := fetchURL(ctx, args.URL)
		if err != nil {
			return "Error: " + err.Error()
		}
		return truncateText(out, maxToolOutputChars)

	default:
		if r.mcp != nil && r.mcp.HasTool(call.Function.Name) {
			result, err := r.mcp.HandleToolCall(call.Function.Name, call.Function.Arguments)
			if err != nil {
				return "Error: " + err.Error()
			}
			return truncateText(result, maxToolOutputChars)
		}
		return "Error: unknown tool " + call.Function.Name
	}
}

func (r *Runner) resolveWorkspacePath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return r.workDir, nil
	}

	candidate := path
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(r.workDir, candidate)
	}
	candidate, err := filepath.Abs(filepath.Clean(candidate))
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}

	if resolved, err := filepath.EvalSymlinks(candidate); err == nil {
		candidate = resolved
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("resolve path: %w", err)
	}

	// Resolve the workspace root the same way so that a root reached via a
	// symlink (e.g. /tmp -> /private/tmp on macOS) compares cleanly against
	// the resolved candidate instead of falsely "escaping".
	base := r.workDir
	if rb, err := filepath.EvalSymlinks(base); err == nil {
		base = rb
	}

	rel, err := filepath.Rel(base, candidate)
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path %q escapes the current working directory", path)
	}
	return candidate, nil
}

func (r *Runner) relativePath(path string) string {
	rel, err := filepath.Rel(r.workDir, path)
	if err != nil || rel == "." {
		return path
	}
	return rel
}

func (r *Runner) readFile(path string, offset, limit int) (string, error) {
	resolved, err := r.resolveWorkspacePath(path)
	if err != nil {
		return "", err
	}

	f, err := os.Open(resolved)
	if err != nil {
		return "", fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return "", fmt.Errorf("stat file: %w", err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("%s is a directory", resolved)
	}

	data, err := io.ReadAll(io.LimitReader(f, maxReadFileChars+1))
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}

	if bytes.IndexByte(data, 0) >= 0 {
		return "", fmt.Errorf("binary file content is not supported by chat tools; use gpterminal read for richer file analysis")
	}

	truncated := len(data) > maxReadFileChars
	if truncated {
		data = data[:maxReadFileChars]
	}

	content := string(data)
	header := fmt.Sprintf("File: %s", resolved)

	// Line-based paging: offset/limit are 1-indexed and optional.
	if offset > 0 || limit > 0 {
		sliced, startLine, endLine := sliceLines(content, offset, limit)
		content = sliced
		header = fmt.Sprintf("File: %s (lines %d-%d)", resolved, startLine, endLine)
	} else if truncated {
		content += "\n...[truncated]"
	}

	return fmt.Sprintf("%s\n\n%s", header, content), nil
}

// sliceLines returns the subset of content covering 1-indexed lines
// [offset, offset+limit-1]. offset<=0 starts at line 1; limit<=0 means no
// upper bound. It returns the sliced text plus the inclusive start and end
// line numbers actually shown.
func sliceLines(content string, offset, limit int) (string, int, int) {
	lines := strings.Split(content, "\n")
	start := 1
	if offset > 1 {
		start = offset
	}
	startIdx := start - 1
	if startIdx > len(lines) {
		startIdx = len(lines)
	}
	endIdx := len(lines)
	if limit > 0 {
		endIdx = startIdx + limit
		if endIdx > len(lines) {
			endIdx = len(lines)
		}
	}
	if startIdx > endIdx {
		startIdx = endIdx
	}
	out := strings.Join(lines[startIdx:endIdx], "\n")
	endLine := endIdx
	if endLine < start {
		endLine = start
	}
	return out, start, endLine
}

func (r *Runner) listDirectory(path string) (string, error) {
	resolved, err := r.resolveWorkspacePath(path)
	if err != nil {
		return "", err
	}
	entries, err := os.ReadDir(resolved)
	if err != nil {
		return "", fmt.Errorf("read directory: %w", err)
	}

	var b strings.Builder
	b.WriteString("Directory: " + resolved + "\n")
	limit := len(entries)
	if limit > maxListEntries {
		limit = maxListEntries
	}
	for i := 0; i < limit; i++ {
		entry := entries[i]
		kind := "file"
		if entry.IsDir() {
			kind = "dir"
		}
		b.WriteString(fmt.Sprintf("- [%s] %s\n", kind, entry.Name()))
	}
	if len(entries) > maxListEntries {
		b.WriteString(fmt.Sprintf("... and %d more entries\n", len(entries)-maxListEntries))
	}
	return strings.TrimSpace(b.String()), nil
}

func (r *Runner) searchText(pattern, path string) (string, error) {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return "", fmt.Errorf("pattern cannot be empty")
	}
	searchPath, err := r.resolveWorkspacePath(path)
	if err != nil {
		return "", err
	}

	if rgPath, err := exec.LookPath("rg"); err == nil {
		out, execErr := exec.Command(rgPath, "-n", "--hidden", "--glob", "!.git", pattern, searchPath).CombinedOutput()
		if execErr != nil {
			var exitErr *exec.ExitError
			if ok := errorAs(execErr, &exitErr); ok && exitErr.ExitCode() == 1 {
				return "No matches found.", nil
			}
			return "", fmt.Errorf("search text: %s", strings.TrimSpace(string(out)))
		}
		return truncateText(string(out), maxToolOutputChars), nil
	}

	out, execErr := exec.Command("grep", "-R", "-n", pattern, searchPath).CombinedOutput()
	if execErr != nil {
		var exitErr *exec.ExitError
		if ok := errorAs(execErr, &exitErr); ok && exitErr.ExitCode() == 1 {
			return "No matches found.", nil
		}
		return "", fmt.Errorf("search text: %s", strings.TrimSpace(string(out)))
	}
	return truncateText(string(out), maxToolOutputChars), nil
}

func (r *Runner) globFiles(pattern, path string) (string, error) {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return "", fmt.Errorf("pattern cannot be empty")
	}
	root, err := r.resolveWorkspacePath(path)
	if err != nil {
		return "", err
	}

	re, err := globToRegex(pattern)
	if err != nil {
		return "", fmt.Errorf("invalid glob pattern: %w", err)
	}

	skipDirs := map[string]bool{
		".git": true, "node_modules": true, ".hg": true, ".svn": true,
		"vendor": true, ".venv": true, "__pycache__": true, "dist": true, "build": true,
	}

	var matches []string
	walkErr := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if p != root && skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			if p != root && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if re.MatchString(rel) {
			matches = append(matches, rel)
		}
		if len(matches) >= maxGlobResults {
			return errGlobLimitReached
		}
		return nil
	})
	if walkErr != nil && !errors.Is(walkErr, errGlobLimitReached) {
		return "", fmt.Errorf("glob: %w", walkErr)
	}

	if len(matches) == 0 {
		return "No files matched.", nil
	}
	sort.Strings(matches)
	out := strings.Join(matches, "\n")
	if len(matches) >= maxGlobResults {
		out += fmt.Sprintf("\n... and possibly more (capped at %d results)", maxGlobResults)
	}
	return out, nil
}

var errGlobLimitReached = errors.New("glob result limit reached")

// globToRegex compiles a shell-style glob into a regexp that matches full
// slash-separated relative paths. Supports: ** (any path, including
// separators), * (any run within a path segment), ? (single char), and
// {a,b,c} alternations. Patterns are anchored to the whole path.
func globToRegex(pattern string) (*regexp.Regexp, error) {
	var b strings.Builder
	b.WriteString("^")
	i := 0
	for i < len(pattern) {
		c := pattern[i]
		switch c {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				b.WriteString(".*")
				i += 2
				// tolerate **/ and /** forms
				if i < len(pattern) && pattern[i] == '/' {
					i++
				}
				continue
			}
			b.WriteString("[^/]*")
		case '?':
			b.WriteString("[^/]")
		case '.':
			b.WriteString(`\.`)
		case '+', '(', ')', '^', '$', '|', '\\':
			b.WriteByte('\\')
			b.WriteByte(c)
		case '{':
			// translate {a,b,c} to (a|b|c), escaping inner metachars naively
			end := strings.IndexByte(pattern[i:], '}')
			if end < 0 {
				return nil, fmt.Errorf("unbalanced '{' in pattern")
			}
			inner := pattern[i+1 : i+end]
			parts := strings.Split(inner, ",")
			b.WriteByte('(')
			for j, p := range parts {
				if j > 0 {
					b.WriteByte('|')
				}
				ep, err := escapeGlobSegment(p)
				if err != nil {
					return nil, err
				}
				b.WriteString(ep)
			}
			b.WriteByte(')')
			i += end
		default:
			b.WriteByte(c)
		}
		i++
	}
	b.WriteString("$")
	return regexp.Compile(b.String())
}

// escapeGlobSegment escapes a literal segment inside a {} group while still
// allowing its own * / ? wildcards.
func escapeGlobSegment(s string) (string, error) {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '*':
			b.WriteString("[^/]*")
		case '?':
			b.WriteString("[^/]")
		case '.', '+', '(', ')', '^', '$', '|', '\\':
			b.WriteByte('\\')
			b.WriteByte(c)
		default:
			b.WriteByte(c)
		}
	}
	return b.String(), nil
}

func (r *Runner) runCommand(ctx context.Context, command string, opts StreamOptions) (string, error) {
	args, err := parseSafeCommand(command)
	if err != nil {
		return "", err
	}
	if err := r.validateCommandArgs(args); err != nil {
		return "", err
	}

	if err := validateSafeCommand(args); err == nil {
		return r.executeCommandArgs(command, args)
	}

	if !opts.AllowWriteTools {
		return "", fmt.Errorf("command %q is not allowed in read-only chat mode", args[0])
	}
	// Commands not in the writable allowlist still go through the approval
	// flow so the user can approve arbitrary commands (e.g. fastfetch).
	if opts.ApproveCommand == nil {
		return "", fmt.Errorf("write-capable commands require an approval-capable chat session")
	}

	rr, riskErr := risk.Evaluate(ctx, command)
	decision, err := opts.ApproveCommand(CommandApprovalRequest{
		Command: command,
		Risk:    rr,
		RiskErr: riskErr,
	})
	if err != nil {
		return "", err
	}
	if !decision.Approved {
		return "Command rejected by user.", nil
	}

	return r.executeCommandArgs(command, args)
}

func (r *Runner) executeCommandArgs(command string, args []string) (string, error) {
	if r.hooks != nil {
		r.hooks.Fire(context.Background(), hooks.PreCommand, &hooks.CommandContext{
			Command: command,
			Args:    args,
			WorkDir: r.workDir,
		})
	}

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = r.workDir
	out, runErr := cmd.CombinedOutput()

	result := strings.TrimSpace(string(out))
	if result == "" {
		result = "(no output)"
	}

	var exitCode int
	if runErr == nil {
		exitCode = 0
	} else if exitErr, ok := runErr.(*exec.ExitError); ok {
		exitCode = exitErr.ExitCode()
	} else {
		return "", fmt.Errorf("run command: %w", runErr)
	}

	if r.hooks != nil {
		r.hooks.Fire(context.Background(), hooks.PostCommand, &hooks.CommandResult{
			Command:  command,
			ExitCode: exitCode,
			Output:   result,
			Err:      runErr,
		})
	}

	return truncateText(fmt.Sprintf("Command: %s\nExit code: %d\nOutput:\n%s", strings.TrimSpace(command), exitCode, result), maxToolOutputChars), nil
}

func (r *Runner) writeFile(path, content string, opts StreamOptions) (string, error) {
	if !opts.AllowWriteTools {
		return "", fmt.Errorf("write_file is not available in read-only chat mode")
	}
	if opts.ApproveFileWrite == nil {
		return "", fmt.Errorf("write_file requires an approval-capable chat session")
	}

	resolved, err := r.resolveWorkspacePath(path)
	if err != nil {
		return "", err
	}

	existing := ""
	exists := false
	if _, err := os.Stat(resolved); err == nil {
		exists = true
		existing, err = fileutil.ReadText(resolved)
		if err != nil {
			return "", err
		}
	} else if !os.IsNotExist(err) {
		return "", err
	}

	diff := diffutil.Unified(r.relativePath(resolved), existing, content)
	if strings.TrimSpace(diff) == "No changes." {
		return "No changes required.", nil
	}

	decision, err := opts.ApproveFileWrite(FileWriteApprovalRequest{
		Path:     r.relativePath(resolved),
		Diff:     diff,
		Existing: exists,
	})
	if err != nil {
		return "", err
	}
	if !decision.Approved {
		return "Write rejected by user.", nil
	}

	if err := fileutil.WriteText(resolved, content); err != nil {
		return "", err
	}

	return truncateText(fmt.Sprintf("Wrote file: %s\nDiff:\n%s", resolved, diff), maxToolOutputChars), nil
}

type editOp struct {
	OldText string `json:"old_text"`
	NewText string `json:"new_text"`
}

func (r *Runner) editFile(path string, edits []editOp, opts StreamOptions) (string, error) {
	if !opts.AllowWriteTools {
		return "", fmt.Errorf("edit_file is not available in read-only chat mode")
	}
	if opts.ApproveFileWrite == nil {
		return "", fmt.Errorf("edit_file requires an approval-capable chat session")
	}
	if len(edits) == 0 {
		return "", fmt.Errorf("no edits provided")
	}

	resolved, err := r.resolveWorkspacePath(path)
	if err != nil {
		return "", err
	}

	existing, err := fileutil.ReadText(resolved)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}

	modified := existing
	for i, edit := range edits {
		count := strings.Count(modified, edit.OldText)
		if count == 0 {
			return "", fmt.Errorf("edit %d: old_text not found in file", i+1)
		}
		if count > 1 {
			return "", fmt.Errorf("edit %d: old_text matches %d locations (must be unique)", i+1, count)
		}
		modified = strings.Replace(modified, edit.OldText, edit.NewText, 1)
	}

	if modified == existing {
		return "No changes required.", nil
	}

	diff := diffutil.Unified(r.relativePath(resolved), existing, modified)
	decision, err := opts.ApproveFileWrite(FileWriteApprovalRequest{
		Path:     r.relativePath(resolved),
		Diff:     diff,
		Existing: true,
	})
	if err != nil {
		return "", err
	}
	if !decision.Approved {
		return "Edit rejected by user.", nil
	}

	if err := fileutil.WriteText(resolved, modified); err != nil {
		return "", err
	}

	return truncateText(fmt.Sprintf("Edited file: %s (%d edit(s) applied)\nDiff:\n%s", r.relativePath(resolved), len(edits), diff), maxToolOutputChars), nil
}

type langConfig struct {
	binary string
	subcmd string // non-empty means prepend as argument (e.g. "run" for `go run`)
	ext    string
}

var langMap = map[string]langConfig{
	"python":     {binary: "python3", ext: ".py"},
	"python3":    {binary: "python3", ext: ".py"},
	"javascript": {binary: "node", ext: ".js"},
	"js":         {binary: "node", ext: ".js"},
	"node":       {binary: "node", ext: ".js"},
	"bash":       {binary: "bash", ext: ".sh"},
	"sh":         {binary: "bash", ext: ".sh"},
	"ruby":       {binary: "ruby", ext: ".rb"},
	"go":         {binary: "go", subcmd: "run", ext: ".go"},
}

func (r *Runner) executeCode(ctx context.Context, language, code string, opts StreamOptions) (string, error) {
	if !opts.AllowWriteTools {
		return "", fmt.Errorf("execute_code is not available in read-only chat mode")
	}
	if opts.ApproveCommand == nil {
		return "", fmt.Errorf("execute_code requires an approval-capable chat session")
	}

	lang := strings.ToLower(strings.TrimSpace(language))
	lc, ok := langMap[lang]
	if !ok {
		return "", fmt.Errorf("unsupported language %q; supported: python, javascript, bash, ruby, go", language)
	}

	binPath, err := exec.LookPath(lc.binary)
	if err != nil {
		return "", fmt.Errorf("interpreter %q not found: %w", lc.binary, err)
	}

	tmp, err := os.CreateTemp("", "gpterminal-code-*"+lc.ext)
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.WriteString(code); err != nil {
		tmp.Close()
		return "", fmt.Errorf("write temp file: %w", err)
	}
	tmp.Close()

	displayCmd := lc.binary
	if lc.subcmd != "" {
		displayCmd += " " + lc.subcmd
	}
	displayCmd += " " + tmpName

	decision, err := opts.ApproveCommand(CommandApprovalRequest{
		Command: displayCmd,
	})
	if err != nil {
		return "", err
	}
	if !decision.Approved {
		return "Code execution rejected by user.", nil
	}

	var cmdArgs []string
	if lc.subcmd != "" {
		cmdArgs = []string{binPath, lc.subcmd, tmpName}
	} else {
		cmdArgs = []string{binPath, tmpName}
	}

	cmd := exec.CommandContext(ctx, cmdArgs[0], cmdArgs[1:]...)
	cmd.Dir = r.workDir
	out, runErr := cmd.CombinedOutput()

	result := strings.TrimSpace(string(out))
	if result == "" {
		result = "(no output)"
	}

	var exitCode int
	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return "", fmt.Errorf("execute code: %w", runErr)
		}
	}

	return truncateText(fmt.Sprintf("[%s]\nExit code: %d\nOutput:\n%s", lang, exitCode, result), maxToolOutputChars), nil
}

func mergeToolCalls(dest map[int]*openai.ToolCall, deltas []openai.ToolCall) {
	for i, delta := range deltas {
		index := i
		if delta.Index != nil {
			index = *delta.Index
		}
		call := dest[index]
		if call == nil {
			call = &openai.ToolCall{Type: openai.ToolTypeFunction}
			dest[index] = call
		}
		if delta.ID != "" {
			call.ID = delta.ID
		}
		if delta.Type != "" {
			call.Type = delta.Type
		}
		if delta.Function.Name != "" {
			call.Function.Name = delta.Function.Name
		}
		if delta.Function.Arguments != "" {
			call.Function.Arguments += delta.Function.Arguments
		}
	}
}

func orderedToolCalls(src map[int]*openai.ToolCall) []openai.ToolCall {
	if len(src) == 0 {
		return nil
	}
	indexes := make([]int, 0, len(src))
	for idx := range src {
		indexes = append(indexes, idx)
	}
	sort.Ints(indexes)

	out := make([]openai.ToolCall, 0, len(indexes))
	for _, idx := range indexes {
		if call := src[idx]; call != nil {
			out = append(out, *call)
		}
	}
	return out
}

func parseSafeCommand(command string) ([]string, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return nil, fmt.Errorf("command cannot be empty")
	}
	args, err := shellSplit(command)
	if err != nil {
		return nil, err
	}
	if len(args) == 0 {
		return nil, fmt.Errorf("command cannot be empty")
	}
	return args, nil
}

// shellSplit tokenizes a command string the way a POSIX shell would split
// words, honoring single quotes, double quotes, and backslash escapes so
// that arguments containing spaces survive (e.g. git commit -m "fix: x").
// Any shell operator (| & ; > < ` ( ) $ or newline) that appears UNQUOTED is
// rejected, which lets callers safely exec args[0] without invoking a shell.
func shellSplit(command string) ([]string, error) {
	const operators = "|&;><`()$\n"
	var (
		args     []string
		cur      strings.Builder
		inSingle bool
		inDouble bool
		haveTok  bool
	)
	flush := func() {
		if haveTok {
			args = append(args, cur.String())
			cur.Reset()
			haveTok = false
		}
	}
	for i := 0; i < len(command); i++ {
		c := command[i]
		switch {
		case inSingle:
			if c == '\'' {
				inSingle = false
			} else {
				cur.WriteByte(c)
			}
		case inDouble:
			switch {
			case c == '"':
				inDouble = false
			case c == '\\' && i+1 < len(command):
				next := command[i+1]
				if next == '"' || next == '\\' || next == '$' || next == '`' || next == '\n' {
					cur.WriteByte(next)
					i++
				} else {
					cur.WriteByte(c)
				}
			default:
				cur.WriteByte(c)
			}
		case c == '\'':
			inSingle = true
			haveTok = true // empty '' still forms a token
		case c == '"':
			inDouble = true
			haveTok = true
		case c == '\\':
			if i+1 < len(command) {
				cur.WriteByte(command[i+1])
				i++
				haveTok = true
			}
		case strings.IndexByte(operators, c) >= 0:
			return nil, fmt.Errorf("shell operators are not allowed in chat tools")
		case c == ' ' || c == '\t':
			flush()
		default:
			cur.WriteByte(c)
			haveTok = true
		}
	}
	if inSingle || inDouble {
		return nil, fmt.Errorf("unterminated quote in command")
	}
	flush()
	return args, nil
}

func validateSafeCommand(args []string) error {
	base := args[0]
	switch base {
	case "pwd", "ls", "cat", "head", "tail", "wc", "file", "stat", "find", "grep", "rg", "uname", "whoami", "id", "ps", "df", "du", "which":
		return nil
	case "git":
		if len(args) < 2 {
			return fmt.Errorf("git subcommand required")
		}
		switch args[1] {
		case "status", "diff", "log", "show", "branch", "rev-parse", "ls-files":
			return nil
		default:
			return fmt.Errorf("git subcommand %q is not allowed in chat tools", args[1])
		}
	default:
		return fmt.Errorf("command %q is not allowed in chat tools", base)
	}
}

func (r *Runner) validateCommandArgs(args []string) error {
	for _, arg := range args[1:] {
		if arg == "" || strings.HasPrefix(arg, "-") {
			continue
		}

		looksLikePath := arg == "." || arg == ".." || filepath.IsAbs(arg) || strings.Contains(arg, string(os.PathSeparator))
		if !looksLikePath {
			if _, err := os.Lstat(filepath.Join(r.workDir, arg)); err == nil {
				looksLikePath = true
			}
		}
		if !looksLikePath {
			continue
		}

		if _, err := r.resolveWorkspacePath(arg); err != nil {
			return fmt.Errorf("command path %q is not allowed: %w", arg, err)
		}
	}
	return nil
}

func truncateText(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	return s[:limit] + "\n...[truncated]"
}

func errorAs(err error, target any) bool { return errors.As(err, target) }
