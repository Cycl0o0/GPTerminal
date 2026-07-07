package codetui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cycl0o0/GPTerminal/internal/chatutil"
	"github.com/cycl0o0/GPTerminal/internal/config"
	"github.com/cycl0o0/GPTerminal/internal/session"
	openai "github.com/sashabaranov/go-openai"
)

// Update is the bubbletea event handler.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.handleResize(msg)
	case tea.KeyMsg:
		return m.handleKey(msg)
	case contentMsg:
		m.streamBuf += msg.delta
		m.refreshViewport()
		return m, waitForEvent(m.eventCh)
	case thinkingMsg:
		m.thinkingBuf += msg.delta
		m.statusLine = "thinking…"
		m.refreshViewport()
		return m, waitForEvent(m.eventCh)
	case toolCallMsg:
		m.flushStreamBuf()
		m.entries = append(m.entries, transcriptEntry{kind: entryTool, label: msg.name, body: firstLine(msg.args, 200)})
		m.statusLine = "running " + msg.name + "…"
		m.refreshViewport()
		return m, waitForEvent(m.eventCh)
	case toolResultMsg:
		m.attachToolResult(msg.name, msg.result)
		m.refreshViewport()
		return m, waitForEvent(m.eventCh)
	case approvalRequestedMsg:
		return m.handleApprovalRequest(msg)
	case streamDoneMsg:
		return m.handleStreamDone(msg)
	case streamCanceledMsg:
		m.finishStream()
		m.entries = append(m.entries, transcriptEntry{kind: entrySystem, label: "system", body: "Canceled."})
		m.refreshViewport()
		return m, nil
	case streamErrMsg:
		m.finishStream()
		m.err = msg.err
		m.entries = append(m.entries, transcriptEntry{kind: entryError, label: "error", body: msg.err.Error()})
		m.refreshViewport()
		return m, nil
	}

	var cmd tea.Cmd
	if !m.streaming || m.pending != nil {
		m.textarea, cmd = m.textarea.Update(msg)
	}
	return m, cmd
}

func (m Model) handleResize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	m.width, m.height = msg.Width, msg.Height
	vpHeight := msg.Height - 6 // header(1)+divider(1)+status(1)+input(3)
	if vpHeight < 3 {
		vpHeight = 3
	}
	if !m.ready {
		m.viewport = newViewport(msg.Width, vpHeight)
		m.ready = true
	} else {
		m.viewport.Width = msg.Width
		m.viewport.Height = vpHeight
	}
	m.textarea.SetWidth(msg.Width - 4)
	m.updateRenderer(msg.Width - 6)
	m.refreshViewport()
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Approval modal captures y/n/a hotkeys.
	if m.pending != nil {
		switch strings.ToLower(msg.String()) {
		case "y", "enter":
			return m.answerApproval(chatutil.ApprovalDecision{Approved: true})
		case "n", "esc":
			return m.answerApproval(chatutil.ApprovalDecision{Approved: false})
		case "a":
			if m.pending.allowAuto {
				m.autoApprove = true
				return m.answerApproval(chatutil.ApprovalDecision{Approved: true, AutoApprove: true})
			}
		}
		return m, nil
	}

	switch msg.Type {
	case tea.KeyCtrlC:
		m.saveSession()
		m.quitting = true
		if m.streamCancel != nil {
			m.streamCancel()
		}
		return m, tea.Quit
	case tea.KeyEsc:
		// Cancel an in-flight turn (does not quit).
		if m.streaming && m.streamCancel != nil {
			m.streamCancel()
			m.statusLine = "canceling…"
		}
		return m, nil
	case tea.KeyCtrlJ:
		m.textarea.InsertString("\n")
		return m, nil
	case tea.KeyEnter:
		if m.streaming {
			return m, nil // ignore Enter mid-stream
		}
		text := strings.TrimSpace(m.textarea.Value())
		if text == "" {
			return m, nil
		}
		if strings.HasPrefix(text, "/") {
			m.textarea.Reset()
			return m.handleSlash(text)
		}
		m.textarea.Reset()
		return m.startTurn(text)
	}

	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	return m, cmd
}

// startTurn appends the user message and launches the stream goroutine.
func (m Model) startTurn(text string) (tea.Model, tea.Cmd) {
	m.entries = append(m.entries, transcriptEntry{kind: entryUser, label: "you", body: text, ts: time.Now().Format("15:04")})
	m.transcript = append(m.transcript, session.ChatMessage{Role: openai.ChatMessageRoleUser, Content: text, Timestamp: time.Now().Format("15:04")})
	m.history = append(m.history, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: text})

	m.streaming = true
	m.streamBuf = ""
	m.thinkingBuf = ""
	m.err = nil
	m.statusLine = "working…"
	m.refreshViewport()

	return m, m.startStream()
}

// startStream spawns the runner goroutine and returns the event pump Cmd. The
// approval callbacks select on ctx.Done() so a cancel never leaves the runner
// goroutine blocked (the bug in the chat TUI).
func (m *Model) startStream() tea.Cmd {
	history := append([]openai.ChatCompletionMessage(nil), m.history...)
	m.eventCh = make(chan tea.Msg, 64)
	m.approvalCh = make(chan chatutil.ApprovalDecision)
	ctx, cancel := context.WithCancel(context.Background())
	m.streamCancel = cancel

	eventCh := m.eventCh
	approvalCh := m.approvalCh

	waitApproval := func(req approvalRequestedMsg) (chatutil.ApprovalDecision, error) {
		select {
		case eventCh <- req:
		case <-ctx.Done():
			return chatutil.ApprovalDecision{Approved: false}, ctx.Err()
		}
		select {
		case dec := <-approvalCh:
			return dec, nil
		case <-ctx.Done():
			return chatutil.ApprovalDecision{Approved: false}, ctx.Err()
		}
	}

	go func() {
		full, finalHistory, err := m.runner.Stream(ctx, history, chatutil.StreamOptions{
			AllowWriteTools: true,
			LiveContent:     true,
			ApprovalMode:    m.opts.ApprovalMode,
			OnThinking:      func(t string) { sendEvent(ctx, eventCh, thinkingMsg{delta: t}) },
			OnContent:       func(c string) { sendEvent(ctx, eventCh, contentMsg{delta: c}) },
			OnToolCall:      func(n, a string) { sendEvent(ctx, eventCh, toolCallMsg{name: n, args: a}) },
			OnToolResult:    func(n, r string) { sendEvent(ctx, eventCh, toolResultMsg{name: n, result: r}) },
			ApproveCommand: func(r chatutil.CommandApprovalRequest) (chatutil.ApprovalDecision, error) {
				return waitApproval(approvalRequestedMsg{
					kind:      "command",
					command:   r.Command,
					risk:      formatRisk(r),
					allowAuto: r.RiskErr == nil && r.Risk != nil && r.Risk.Score <= 7,
				})
			},
			ApproveFileWrite: func(r chatutil.FileWriteApprovalRequest) (chatutil.ApprovalDecision, error) {
				return waitApproval(approvalRequestedMsg{kind: "file_write", path: r.Path, diff: r.Diff})
			},
		})
		// The terminal message MUST be delivered so the UI unwinds (finishStream
		// runs only from a terminal-message handler). It uses a plain blocking
		// send, never the ctx-aware sendEvent: on the cancel path ctx is already
		// Done, and a ctx-select would race the send against ctx.Done() and drop
		// the message ~50% of the time, wedging the TUI in "canceling…". The
		// buffered channel (cap 64) plus the always-outstanding waitForEvent
		// receiver guarantee this send lands.
		var terminal tea.Msg
		switch {
		case err != nil && ctx.Err() != nil:
			terminal = streamCanceledMsg{}
		case err != nil:
			terminal = streamErrMsg{err: err}
		default:
			terminal = streamDoneMsg{content: full, history: finalHistory}
		}
		eventCh <- terminal
	}()

	return waitForEvent(m.eventCh)
}

func (m Model) handleApprovalRequest(msg approvalRequestedMsg) (tea.Model, tea.Cmd) {
	// Auto-approve short-circuit for commands the user opted into.
	if m.autoApprove && msg.kind == "command" && msg.allowAuto {
		select {
		case m.approvalCh <- chatutil.ApprovalDecision{Approved: true, AutoApprove: true}:
		default:
		}
		return m, waitForEvent(m.eventCh)
	}
	m.pending = &approvalState{kind: msg.kind, command: msg.command, path: msg.path, diff: msg.diff, risk: msg.risk, allowAuto: msg.allowAuto}
	m.refreshViewport()
	return m, waitForEvent(m.eventCh)
}

func (m Model) answerApproval(dec chatutil.ApprovalDecision) (tea.Model, tea.Cmd) {
	if m.approvalCh != nil {
		select {
		case m.approvalCh <- dec:
		default:
		}
	}
	m.pending = nil
	m.statusLine = "working…"
	m.refreshViewport()
	return m, waitForEvent(m.eventCh)
}

func (m Model) handleStreamDone(msg streamDoneMsg) (tea.Model, tea.Cmd) {
	m.finishStream()
	m.history = msg.history
	if strings.TrimSpace(msg.content) != "" {
		m.entries = append(m.entries, transcriptEntry{kind: entryAI, label: "agent", body: msg.content, ts: time.Now().Format("15:04")})
		m.transcript = append(m.transcript, session.ChatMessage{Role: openai.ChatMessageRoleAssistant, Content: msg.content, Timestamp: time.Now().Format("15:04")})
	}
	m.saveSession()
	m.refreshViewport()
	return m, nil
}

func (m *Model) finishStream() {
	m.streaming = false
	m.statusLine = ""
	m.streamBuf = ""
	m.thinkingBuf = ""
	m.pending = nil
	if m.streamCancel != nil {
		m.streamCancel()
		m.streamCancel = nil
	}
	m.eventCh = nil
	m.approvalCh = nil
}

// flushStreamBuf converts the accumulated live text into a durable AI entry so
// interleaved tool calls read in order.
func (m *Model) flushStreamBuf() {
	if strings.TrimSpace(m.streamBuf) != "" {
		m.entries = append(m.entries, transcriptEntry{kind: entryAI, label: "agent", body: m.streamBuf})
		m.streamBuf = ""
	}
}

func (m *Model) attachToolResult(name, result string) {
	ok := !strings.HasPrefix(strings.TrimSpace(result), "Error")
	// Attach to the most recent matching tool entry.
	for i := len(m.entries) - 1; i >= 0; i-- {
		if m.entries[i].kind == entryTool && m.entries[i].label == name && m.entries[i].toolRes == "" {
			m.entries[i].toolOK = ok
			m.entries[i].toolRes = firstLine(result, 200)
			m.statusLine = ""
			return
		}
	}
	m.entries = append(m.entries, transcriptEntry{kind: entryTool, label: name, toolOK: ok, toolRes: firstLine(result, 200)})
}

func (m Model) saveSession() {
	if m.sessionName == "" {
		return
	}
	_ = session.Save(&session.Record{
		Kind: session.KindCode,
		Name: m.sessionName,
		Chat: &session.ChatData{Transcript: m.transcript, History: m.history},
	})
}

// waitForEvent drains one message from the bridge channel. Nil-safe so a
// finished stream doesn't panic the loop.
func waitForEvent(ch chan tea.Msg) tea.Cmd {
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return nil
		}
		return msg
	}
}

// sendEvent pushes an event unless the turn was canceled, avoiding a blocked
// send on a full channel after cancellation (goroutine-leak guard).
func sendEvent(ctx context.Context, ch chan tea.Msg, msg tea.Msg) {
	select {
	case ch <- msg:
	case <-ctx.Done():
	}
}

func formatRisk(r chatutil.CommandApprovalRequest) string {
	if r.RiskErr != nil {
		return "risk unavailable"
	}
	if r.Risk == nil {
		return ""
	}
	return fmt.Sprintf("%d/10 [%s] %s", r.Risk.Score, strings.ToUpper(r.Risk.Level), r.Risk.Summary)
}

func firstLine(s string, max int) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if len(s) > max {
		s = s[:max] + "…"
	}
	return s
}

// handleSlash dispatches in-TUI slash commands. Returns the model + optional Cmd.
func (m Model) handleSlash(text string) (tea.Model, tea.Cmd) {
	fields := strings.Fields(text)
	cmd := strings.ToLower(fields[0])
	switch cmd {
	case "/quit", "/exit", "/q":
		m.saveSession()
		m.quitting = true
		return m, tea.Quit
	case "/clear":
		m.entries = nil
		m.transcript = nil
		// Keep the system prompt if present; for providers with no seeded
		// system message (OpenClaw) history may be empty, so guard the reslice.
		if len(m.history) > 0 {
			m.history = m.history[:1]
		}
		m.refreshViewport()
	case "/model":
		if len(fields) > 1 {
			config.SetActiveModel(fields[1])
			m.entries = append(m.entries, sysEntry("model set to "+fields[1]))
		} else {
			m.entries = append(m.entries, sysEntry("model: "+config.Model()))
		}
		m.refreshViewport()
	case "/effort":
		if len(fields) > 1 {
			config.SetActiveEffort(fields[1])
			m.entries = append(m.entries, sysEntry("effort set to "+fields[1]))
		} else {
			m.entries = append(m.entries, sysEntry("effort: "+config.Effort()))
		}
		m.refreshViewport()
	case "/approval":
		if len(fields) > 1 {
			m.opts.ApprovalMode = chatutil.ParseApprovalMode(fields[1])
			m.entries = append(m.entries, sysEntry("approval mode: "+string(m.opts.ApprovalMode)))
		} else {
			m.entries = append(m.entries, sysEntry("approval mode: "+string(m.opts.effectiveMode())))
		}
		m.refreshViewport()
	case "/help":
		m.entries = append(m.entries, sysEntry(helpText))
		m.refreshViewport()
	default:
		m.entries = append(m.entries, sysEntry("unknown command: "+cmd+"  (/help for list)"))
		m.refreshViewport()
	}
	return m, nil
}

func (o Options) effectiveMode() chatutil.ApprovalMode {
	if o.ApprovalMode == "" {
		return chatutil.ApprovalDefault
	}
	return o.ApprovalMode
}

func sysEntry(body string) transcriptEntry {
	return transcriptEntry{kind: entrySystem, label: "system", body: body}
}

const helpText = "Commands: /model [name] · /effort [level] · /approval [plan|default|auto-edit|yolo] · /clear · /help · /quit\n" +
	"Keys: Enter send · Ctrl-J newline · Esc cancel turn · Ctrl-C quit · y/n/a answer approvals"
