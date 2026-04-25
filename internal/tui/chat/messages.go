package chat

import tea "github.com/charmbracelet/bubbletea"

type StreamChunkMsg struct {
	Content string
}

type StreamDoneMsg struct {
	FullContent string
}

type StreamErrMsg struct {
	Err error
}

type WindowSizeMsg tea.WindowSizeMsg
