package serve

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/cycl0o0/GPTerminal/internal/ai"
	"github.com/cycl0o0/GPTerminal/internal/chatutil"
	"github.com/cycl0o0/GPTerminal/internal/config"
	"github.com/cycl0o0/GPTerminal/internal/mcp"
	"github.com/cycl0o0/GPTerminal/internal/session"
	"github.com/cycl0o0/GPTerminal/internal/system"
	openai "github.com/sashabaranov/go-openai"
)

// Server drives one NDJSON stdio session. It owns a single in-flight prompt at
// a time (the GUI issues them serially per window); concurrent prompts are
// rejected with an error event.
type Server struct {
	in      *bufio.Reader
	outMu   sync.Mutex
	out     io.Writer
	version string

	mu      sync.Mutex
	running bool               // a prompt is in flight
	cancel  context.CancelFunc // cancels the in-flight prompt
	// approvals maps approval_id → the channel the blocked callback waits on.
	approvals map[string]chan chatutil.ApprovalDecision
	seq       int

	// active prompt session state, persisted across turns when Session is set.
	history     []openai.ChatCompletionMessage
	sessionName string
}

// Options configures a Server.
type Options struct {
	In      io.Reader
	Out     io.Writer
	Version string
}

// New builds a Server. In defaults to stdin, Out to stdout.
func New(opts Options) *Server {
	in := opts.In
	if in == nil {
		in = os.Stdin
	}
	out := opts.Out
	if out == nil {
		out = os.Stdout
	}
	return &Server{
		in:        bufio.NewReaderSize(in, 1<<20),
		out:       out,
		version:   opts.Version,
		approvals: map[string]chan chatutil.ApprovalDecision{},
	}
}

// Run reads request frames until EOF or a shutdown request. It never returns an
// error for a bad frame — those are reported as error events — so the loop is
// resilient to a misbehaving client.
func (s *Server) Run(ctx context.Context) error {
	s.emit(Event{Type: EvtReady, SchemaVersion: SchemaVersion, Version: s.version})

	for {
		line, err := s.in.ReadBytes('\n')
		if len(line) > 0 {
			s.handleLine(ctx, line)
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("serve read: %w", err)
		}
	}
}

func (s *Server) handleLine(ctx context.Context, line []byte) {
	trimmed := strings.TrimSpace(string(line))
	if trimmed == "" {
		return
	}
	var req Request
	if err := json.Unmarshal([]byte(trimmed), &req); err != nil {
		s.emit(Event{Type: EvtError, Message: "invalid JSON request: " + err.Error()})
		return
	}

	switch req.Type {
	case ReqPrompt:
		// Run the turn on its own goroutine so the read loop keeps consuming
		// stdin. This is what lets approval_response and cancel frames arrive
		// while the turn is blocked on an approval; running it inline would
		// deadlock the whole server the moment an approval is requested.
		go s.handlePrompt(ctx, req)
	case ReqApprovalResponse:
		s.handleApprovalResponse(req)
	case ReqCancel:
		s.handleCancel()
	case ReqConfigGet:
		s.handleConfigGet(req)
	case ReqConfigSet:
		s.handleConfigSet(req)
	case ReqConfigList:
		s.handleConfigList(req)
	case ReqSessionsList:
		s.handleSessionsList(req)
	case ReqSessionResume:
		s.handleSessionResume(req)
	case ReqModelsList:
		go s.handleModelsList(ctx, req)
	case ReqPing:
		s.emit(Event{Type: EvtPong, RequestID: req.RequestID})
	case ReqShutdown:
		s.emit(Event{Type: EvtDone, RequestID: req.RequestID, StopReason: "shutdown"})
		os.Exit(0)
	default:
		s.emit(Event{Type: EvtError, RequestID: req.RequestID, Message: "unknown request type: " + req.Type})
	}
}

func (s *Server) emit(e Event) {
	data, err := json.Marshal(e)
	if err != nil {
		return
	}
	s.outMu.Lock()
	defer s.outMu.Unlock()
	s.out.Write(data)
	s.out.Write([]byte("\n"))
	if f, ok := s.out.(interface{ Sync() error }); ok {
		_ = f.Sync()
	}
}

func (s *Server) nextID(prefix string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seq++
	return fmt.Sprintf("%s_%d", prefix, s.seq)
}

// handlePrompt runs one agentic turn. It applies per-request model/provider/
// effort/approval overrides (process-scoped) and streams every event.
func (s *Server) handlePrompt(parent context.Context, req Request) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		s.emit(Event{Type: EvtError, RequestID: req.RequestID, Message: "a prompt is already in flight; cancel it first"})
		return
	}
	s.running = true
	ctx, cancel := context.WithCancel(parent)
	s.cancel = cancel
	s.mu.Unlock()

	defer func() {
		cancel()
		s.mu.Lock()
		s.running = false
		s.cancel = nil
		s.mu.Unlock()
	}()

	// Per-request overrides (process-scoped viper, no disk write).
	if req.Provider != "" {
		// Provider selection requires a fresh client; handled below via config.
		config.SetActiveProvider(req.Provider)
	}
	if req.Model != "" {
		config.SetActiveModel(req.Model)
	}
	if req.Effort != "" {
		config.SetActiveEffort(req.Effort)
	}
	if req.CWD != "" {
		if err := os.Chdir(req.CWD); err != nil {
			s.emit(Event{Type: EvtError, RequestID: req.RequestID, Message: "cwd: " + err.Error()})
		}
	}

	client, err := ai.NewClient()
	if err != nil {
		s.emit(Event{Type: EvtError, RequestID: req.RequestID, Message: err.Error(), Fatal: false})
		s.emit(Event{Type: EvtDone, RequestID: req.RequestID, StopReason: "error"})
		return
	}

	sysInfo := system.Detect()
	var mcpReg *mcp.Registry
	if servers := config.MCPServers(); len(servers) > 0 {
		mcpReg = mcp.NewRegistry()
		if err := mcpReg.LoadFromConfig(); err == nil {
			defer mcpReg.Close()
		} else {
			mcpReg = nil
		}
	}
	runner := chatutil.NewRunnerWithMCP(client, sysInfo, mcpReg)

	// Session continuity: reuse in-memory history when the same session name is
	// used across prompts; otherwise seed a fresh code system prompt.
	if req.Session != "" && req.Session == s.sessionName && len(s.history) > 0 {
		// continue
	} else {
		s.sessionName = req.Session
		s.history = seedHistory(client, sysInfo)
		if req.Session != "" {
			if rec, err := session.Load(req.Session); err == nil && rec.Chat != nil && len(rec.Chat.History) > 0 {
				s.history = rec.Chat.History
			}
		}
	}

	s.history = append(s.history, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: req.Text,
	})

	s.emit(Event{
		Type:      EvtSessionStarted,
		RequestID: req.RequestID,
		SessionID: req.Session,
		Version:   s.version,
	})

	opts := s.streamOptions(ctx, req)
	_, finalHistory, streamErr := runner.Stream(ctx, s.history, opts)
	s.history = finalHistory

	if req.Session != "" {
		_ = session.Save(&session.Record{
			Kind: session.KindCode,
			Name: req.Session,
			Chat: &session.ChatData{History: finalHistory},
		})
	}

	if streamErr != nil {
		if ctx.Err() != nil {
			s.emit(Event{Type: EvtDone, RequestID: req.RequestID, StopReason: "canceled"})
			return
		}
		s.emit(Event{Type: EvtError, RequestID: req.RequestID, Message: streamErr.Error()})
		s.emit(Event{Type: EvtDone, RequestID: req.RequestID, StopReason: "error"})
		return
	}
	s.emit(Event{Type: EvtDone, RequestID: req.RequestID, StopReason: "complete"})
}

func seedHistory(client *ai.Client, sysInfo system.SystemInfo) []openai.ChatCompletionMessage {
	if client.IsOpenClaw() {
		return nil
	}
	return []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: ai.CodeSystemPrompt(sysInfo.ContextBlock(), "")},
	}
}

// streamOptions builds the StreamOptions whose callbacks emit protocol events
// and whose approval callbacks block on an approval_response frame.
func (s *Server) streamOptions(ctx context.Context, req Request) chatutil.StreamOptions {
	mode := chatutil.ParseApprovalMode(config.ApprovalMode())
	if req.ApprovalMode != "" {
		mode = chatutil.ParseApprovalMode(req.ApprovalMode)
	}
	rid := req.RequestID
	return chatutil.StreamOptions{
		AllowWriteTools: true,
		LiveContent:     true,
		ApprovalMode:    mode,
		OnThinking: func(t string) {
			s.emit(Event{Type: EvtThinking, RequestID: rid, Delta: t})
		},
		OnContent: func(c string) {
			s.emit(Event{Type: EvtContent, RequestID: rid, Delta: c})
		},
		OnToolCall: func(name, args string) {
			s.emit(Event{Type: EvtToolCall, RequestID: rid, Name: name, Args: args})
		},
		OnToolResult: func(name, result string) {
			s.emit(Event{Type: EvtToolResult, RequestID: rid, Name: name, Result: result})
		},
		ApproveCommand: func(r chatutil.CommandApprovalRequest) (chatutil.ApprovalDecision, error) {
			e := Event{Type: EvtApprovalRequest, RequestID: rid, Kind: "command", Command: r.Command}
			if r.Risk != nil {
				e.Risk = &RiskInfo{Score: r.Risk.Score, Level: r.Risk.Level, Summary: r.Risk.Summary}
			}
			return s.waitApproval(ctx, e)
		},
		ApproveFileWrite: func(r chatutil.FileWriteApprovalRequest) (chatutil.ApprovalDecision, error) {
			return s.waitApproval(ctx, Event{Type: EvtApprovalRequest, RequestID: rid, Kind: "file_write", Path: r.Path, Diff: r.Diff})
		},
	}
}

// waitApproval emits an approval_request and blocks until the matching
// approval_response arrives or the prompt is canceled (via ctx). The ctx case
// guarantees the runner goroutine unwinds even if no cancel frame drains this
// specific approval.
func (s *Server) waitApproval(ctx context.Context, e Event) (chatutil.ApprovalDecision, error) {
	id := s.nextID("approval")
	ch := make(chan chatutil.ApprovalDecision, 1)

	s.mu.Lock()
	s.approvals[id] = ch
	s.mu.Unlock()

	e.ApprovalID = id
	s.emit(e)

	defer func() {
		s.mu.Lock()
		delete(s.approvals, id)
		s.mu.Unlock()
	}()

	select {
	case dec := <-ch:
		return dec, nil
	case <-ctx.Done():
		return chatutil.ApprovalDecision{Approved: false}, ctx.Err()
	}
}

func (s *Server) handleApprovalResponse(req Request) {
	s.mu.Lock()
	ch, ok := s.approvals[req.ApprovalID]
	s.mu.Unlock()
	if !ok {
		s.emit(Event{Type: EvtError, Message: "no pending approval " + req.ApprovalID})
		return
	}
	ch <- chatutil.ApprovalDecision{Approved: req.Approved, AutoApprove: req.AutoApprove}
}

func (s *Server) handleCancel() {
	s.mu.Lock()
	cancel := s.cancel
	// Release every blocked approval with a denial so the runner unwinds.
	for id, ch := range s.approvals {
		select {
		case ch <- chatutil.ApprovalDecision{Approved: false}:
		default:
		}
		delete(s.approvals, id)
	}
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (s *Server) handleConfigGet(req Request) {
	def, ok := config.LookupKey(req.Key)
	if !ok {
		s.emit(Event{Type: EvtError, RequestID: req.RequestID, Message: "unknown setting " + req.Key})
		return
	}
	val := config.GetValue(def.Key)
	if def.Kind == config.KindSecret {
		val = maskSecret(val)
	}
	s.emit(Event{Type: EvtResult, RequestID: req.RequestID, Data: map[string]string{"key": def.Key, "value": val}})
}

func (s *Server) handleConfigSet(req Request) {
	if err := config.SetValue(req.Key, req.Value); err != nil {
		s.emit(Event{Type: EvtError, RequestID: req.RequestID, Message: err.Error()})
		return
	}
	s.emit(Event{Type: EvtResult, RequestID: req.RequestID, Data: map[string]string{"key": req.Key, "value": req.Value}})
}

func (s *Server) handleConfigList(req Request) {
	list := make([]map[string]any, 0, len(config.SettableKeys))
	for _, def := range config.SettableKeys {
		val := config.GetValue(def.Key)
		if def.Kind == config.KindSecret {
			val = maskSecret(val)
		}
		list = append(list, map[string]any{
			"key":   def.Key,
			"value": val,
			"desc":  def.Desc,
			"enum":  def.Enum,
		})
	}
	s.emit(Event{Type: EvtResult, RequestID: req.RequestID, Data: list})
}

func (s *Server) handleSessionsList(req Request) {
	entries, err := session.List()
	if err != nil {
		s.emit(Event{Type: EvtError, RequestID: req.RequestID, Message: err.Error()})
		return
	}
	out := make([]map[string]any, 0, len(entries))
	for _, e := range entries {
		out = append(out, map[string]any{
			"name":       e.Name,
			"kind":       string(e.Kind),
			"updated_at": e.UpdatedAt.Format(time.RFC3339),
			"messages":   e.ChatMessages,
			"preview":    e.LastPreview,
		})
	}
	s.emit(Event{Type: EvtResult, RequestID: req.RequestID, Data: out})
}

// handleSessionResume loads a saved session's history into the server's state
// so the next prompt continues it. It sets s.sessionName so handlePrompt's
// continuity check reuses this history rather than reseeding.
func (s *Server) handleSessionResume(req Request) {
	if req.Session == "" {
		s.emit(Event{Type: EvtError, RequestID: req.RequestID, Message: "session name required"})
		return
	}
	// Refuse to swap session state out from under an in-flight prompt goroutine,
	// which reads s.history/s.sessionName without the lock while it streams.
	s.mu.Lock()
	running := s.running
	s.mu.Unlock()
	if running {
		s.emit(Event{Type: EvtError, RequestID: req.RequestID, Message: "cannot resume a session while a prompt is in flight; cancel it first"})
		return
	}
	rec, err := session.Load(req.Session)
	if err != nil {
		s.emit(Event{Type: EvtError, RequestID: req.RequestID, Message: err.Error()})
		return
	}
	if rec.Chat == nil {
		s.emit(Event{Type: EvtError, RequestID: req.RequestID, Message: "session has no chat history"})
		return
	}
	s.mu.Lock()
	s.sessionName = req.Session
	s.history = rec.Chat.History
	s.mu.Unlock()
	s.emit(Event{Type: EvtResult, RequestID: req.RequestID, Data: map[string]any{
		"session":  req.Session,
		"messages": len(rec.Chat.Transcript),
	}})
}

func (s *Server) handleModelsList(ctx context.Context, req Request) {
	client, err := ai.NewClient()
	if err != nil {
		s.emit(Event{Type: EvtError, RequestID: req.RequestID, Message: err.Error()})
		return
	}
	models, err := client.ListModels(ctx)
	if err != nil {
		s.emit(Event{Type: EvtError, RequestID: req.RequestID, Message: err.Error()})
		return
	}
	s.emit(Event{Type: EvtResult, RequestID: req.RequestID, Data: models})
}

func maskSecret(s string) string {
	if len(s) <= 8 {
		if s == "" {
			return ""
		}
		return "****"
	}
	return s[:4] + "..." + s[len(s)-4:]
}
