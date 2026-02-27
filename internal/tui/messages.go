package tui

// FocusPanel identifies which panel has focus.
type FocusPanel int

const (
	FocusTerminal FocusPanel = iota
	FocusSlack
	FocusGitHub
	FocusSummary
)

// FocusChangedMsg is sent when focus changes between panels.
type FocusChangedMsg struct {
	Panel FocusPanel
}
