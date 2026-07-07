package codetui

import openai "github.com/sashabaranov/go-openai"

// Stream event messages bridged from the chatutil.Runner callbacks onto the
// bubbletea event loop (via a buffered channel drained by waitForEvent).

type contentMsg struct{ delta string }
type thinkingMsg struct{ delta string }
type toolCallMsg struct{ name, args string }
type toolResultMsg struct{ name, result string }

type approvalRequestedMsg struct {
	kind      string // "command" | "file_write"
	command   string
	path      string
	diff      string
	risk      string
	allowAuto bool
}

type streamDoneMsg struct {
	content string
	history []openai.ChatCompletionMessage
}

type streamCanceledMsg struct{}
type streamErrMsg struct{ err error }
