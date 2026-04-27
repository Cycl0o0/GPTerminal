package chat

import (
	tea "github.com/charmbracelet/bubbletea"
	openai "github.com/sashabaranov/go-openai"
)

type StreamChunkMsg struct {
	Content string
}

type ThinkingMsg struct {
	Content string
}

type ToolStatusMsg struct {
	Content string
}

type ApprovalRequestedMsg struct {
	Kind      string
	Command   string
	Path      string
	Diff      string
	Risk      string
	AllowAuto bool
}

type StreamDoneMsg struct {
	FullContent string
	History     []openai.ChatCompletionMessage
}

type StreamCanceledMsg struct{}

type StreamErrMsg struct {
	Err error
}

type WindowSizeMsg tea.WindowSizeMsg
