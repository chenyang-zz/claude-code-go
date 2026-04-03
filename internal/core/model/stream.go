package model

type Event struct {
	Type    string
	Text    string
	ToolUse *ToolUse
}

type Stream <-chan Event
