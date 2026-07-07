# GPTerminal v3.3 — Agentic upgrade design

Derived from a 9-dimension parallel audit. Implemented in dependency order.

## Phase 1 — Foundation bug fixes (everything depends on these)
1. **Anthropic multi-tool-call collapse** (HIGH): `anthropic_provider.go` never sets
   `openai.ToolCall.Index`, so `mergeToolCalls` maps every Claude tool call to index 0 →
   corrupted/dropped calls whenever Claude emits ≥2 tool_use blocks. Set `Index` in the
   provider (content_block_stop + flushPendingToolCalls). Same guard for Gemini.
2. **Atomic session writes**: `session.Save` uses `os.WriteFile` (non-atomic) → interrupt
   corrupts JSON → next load silently discards the whole session. Write temp + `os.Rename`.
   `session.List` aborts on first corrupt file → skip instead.
3. **run_command ignores ctx**: `executeCommandArgs`/`searchText` use `exec.Command` → hung
   command can't be canceled. Use `exec.CommandContext`, flow ctx to hooks.
4. **Approval EOF bypass** (MED-HIGH): approval readers default to Approved on EOF/empty →
   piped/Ctrl-D auto-approves everything. Fail closed (deny on read error).

## Phase 2 — Reasoning effort (per provider)
- Config key `effort` ∈ {none,minimal,low,medium,high,max}; env `GPTERMINAL_EFFORT`.
- OpenAI: set `req.ReasoningEffort`; for o1/o3/o4/gpt-5 use `MaxCompletionTokens` (not MaxTokens)
  and omit temperature (client-side ReasoningValidator rejects otherwise).
- Anthropic: map to `OutputConfig.Effort` (low/medium/high/xhigh/max).
- Gemini: no-op (deprecated SDK lacks ThinkingConfig).
- Carried on the shared `openai.ChatCompletionRequest` (a custom header field) so providers translate.

## Phase 3 — Settings system + approval modes
- New keys: `approval_mode` {plan,default,auto-edit,yolo}, `theme`, `tui` {auto,on,off},
  `max_tool_rounds`, `auto_compact`, `effort`.
- `config.KeyDef` registry → generic `config get/set/list` CLI; `saveAny(key, any)`.
- `chatutil.ApprovalPolicy` computed from approval mode, consulted uniformly (code/chat/agent/TUI):
  - plan: read-only; deny all mutations
  - default: prompt for mutations, auto-run read-only allowlist
  - auto-edit: auto-approve file writes, prompt for commands
  - yolo: auto-approve everything

## Phase 4 — Serve mode (GUI protocol)
- `gpterminal serve --stdio`: long-lived NDJSON process. Requests: prompt/approval_response/
  cancel/config_get/config_set/sessions_list/session_resume/models_list/shutdown. Events:
  session_started/content/thinking/tool_call/tool_result/approval_request/usage/done/error/log.
  Maps 1:1 onto StreamOptions callbacks. Fixes the GUI deadlock (no stdin data-read path).

## Phase 5 — Full code TUI (bubbletea)
- `internal/tui/code`: streaming, tool timeline, diff viewer, approval modal with hotkeys,
  slash commands, model/effort/token status line. Fixes chat TUI leaks (ctx-select on approvalCh,
  cancel unblocks approval). `code.Run` → TUI when TTY, else existing REPL (kept as fallback).

## Phase 6 — Agent plan mode
- `agent --plan`: generate plan artifact → approve → execute steps → summary. Plan stored in
  `AgentData`. Replaces the fragile `[AGENT_DONE]` substring check with a structured done signal.
- `agent --yes` (AutoApprove, previously unreachable from the CLI) and `--approval <mode>`.

## Additional bug fixes folded in
- gptdo `executeCD` used `status=$?` — a read-only special in zsh → cd always "failed". Renamed var.
- OpenClaw stream `Close()` leaked the websocket + could wedge the gateway dispatch goroutine on a
  full channel. Added a done channel + `closeOnce`; the OnEvent send aborts on close.
- Self-update: added an HTTP client timeout and a Windows-safe replace (rename-aside, cleanup `.old`
  on next run) — `os.Rename` over a running `.exe` is Access Denied on Windows.

## Post-implementation adversarial review (fixed)
A 5-dimension review with per-finding verification surfaced and fixed:
1. CRITICAL: serve `handlePrompt` ran inline in the read loop → any approval-needing prompt
   deadlocked. Now runs on a goroutine; `waitApproval` selects on `ctx.Done()`.
2. HIGH: TUI cancel dropped the terminal message ~50% (ctx race) → wedged "canceling…". Terminal
   messages now use a plain blocking send.
3. HIGH: `config set` persisted process-only `SetActive*` overrides and env secrets to disk. `saveAny`
   now round-trips through a file-only viper instance.
4. HIGH: `/clear` panicked on empty history (OpenClaw). Guarded the reslice.
5. LOW: `session_resume` was declared but unhandled; added, and rejected while a prompt is in flight
   (closes a state race).
6. LOW: agent plan mode used a second stdin reader → piped approvals swallowed. Single shared reader.
