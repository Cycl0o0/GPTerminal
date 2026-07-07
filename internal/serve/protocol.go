// Package serve implements a long-lived NDJSON-over-stdio protocol for driving
// the GPTerminal agent from a GUI or other programmatic client (e.g. the
// GPTerminal-GUI VS Code fork). One JSON object per line in each direction.
//
// The client writes request frames to the process's stdin and reads event
// frames from stdout. stderr is reserved for human/diagnostic logging so it
// never corrupts the protocol stream. Every event carries the request_id it
// belongs to (empty for process-level events like "ready").
package serve

// SchemaVersion is bumped when the wire protocol changes incompatibly.
const SchemaVersion = 1

// ---- Requests (client → server, read from stdin) ----

// Request is the envelope for every inbound frame. Only the fields relevant to
// Type are populated.
type Request struct {
	Type      string `json:"type"`
	RequestID string `json:"request_id,omitempty"`

	// prompt
	Text         string `json:"text,omitempty"`
	Session      string `json:"session,omitempty"`
	Model        string `json:"model,omitempty"`
	Provider     string `json:"provider,omitempty"`
	Effort       string `json:"effort,omitempty"`
	ApprovalMode string `json:"approval_mode,omitempty"`
	CWD          string `json:"cwd,omitempty"`

	// approval_response
	ApprovalID  string `json:"approval_id,omitempty"`
	Approved    bool   `json:"approved,omitempty"`
	AutoApprove bool   `json:"auto_approve,omitempty"`

	// config_set
	Key   string `json:"key,omitempty"`
	Value string `json:"value,omitempty"`
}

// Request type constants.
const (
	ReqPrompt           = "prompt"
	ReqApprovalResponse = "approval_response"
	ReqCancel           = "cancel"
	ReqConfigGet        = "config_get"
	ReqConfigSet        = "config_set"
	ReqConfigList       = "config_list"
	ReqSessionsList     = "sessions_list"
	ReqSessionResume    = "session_resume"
	ReqModelsList       = "models_list"
	ReqShutdown         = "shutdown"
	ReqPing             = "ping"
)

// ---- Events (server → client, written to stdout) ----

// Event is the envelope for every outbound frame.
type Event struct {
	Type      string `json:"type"`
	RequestID string `json:"request_id,omitempty"`

	// content / thinking / log
	Delta string `json:"delta,omitempty"`
	Text  string `json:"text,omitempty"`
	Level string `json:"level,omitempty"` // for log: info|warn|error

	// tool_call / tool_result
	CallID  string `json:"call_id,omitempty"`
	Name    string `json:"name,omitempty"`
	Args    string `json:"args,omitempty"`
	Result  string `json:"result,omitempty"`
	IsError bool   `json:"is_error,omitempty"`

	// approval_request
	ApprovalID string    `json:"approval_id,omitempty"`
	Kind       string    `json:"kind,omitempty"` // command | file_write
	Command    string    `json:"command,omitempty"`
	Path       string    `json:"path,omitempty"`
	Diff       string    `json:"diff,omitempty"`
	Risk       *RiskInfo `json:"risk,omitempty"`

	// usage
	InputTokens  int     `json:"input_tokens,omitempty"`
	OutputTokens int     `json:"output_tokens,omitempty"`
	Cost         float64 `json:"cost,omitempty"`

	// session_started / ready
	SessionID       string `json:"session_id,omitempty"`
	SchemaVersion   int    `json:"schema_version,omitempty"`
	Version         string `json:"version,omitempty"`

	// done
	StopReason string `json:"stop_reason,omitempty"`

	// error
	Message string `json:"message,omitempty"`
	Fatal   bool   `json:"fatal,omitempty"`

	// config_get / config_list / sessions_list / models_list results
	Data any `json:"data,omitempty"`
}

// RiskInfo is the risk summary attached to an approval_request.
type RiskInfo struct {
	Score   int    `json:"score"`
	Level   string `json:"level"`
	Summary string `json:"summary"`
}

// Event type constants.
const (
	EvtReady           = "ready"
	EvtSessionStarted  = "session_started"
	EvtContent         = "content"
	EvtThinking        = "thinking"
	EvtToolCall        = "tool_call"
	EvtToolResult      = "tool_result"
	EvtApprovalRequest = "approval_request"
	EvtUsage           = "usage"
	EvtDone            = "done"
	EvtError           = "error"
	EvtLog             = "log"
	EvtResult          = "result" // generic result for config/sessions/models queries
	EvtPong            = "pong"
)
