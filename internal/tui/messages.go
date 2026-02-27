package tui

// FocusPanel identifies which panel has focus.
type FocusPanel int

const (
	FocusTerminal FocusPanel = iota
	FocusSlack
	FocusGitHub
	FocusSummary
)

func (f FocusPanel) String() string {
	switch f {
	case FocusTerminal:
		return "terminal"
	case FocusSlack:
		return "slack"
	case FocusGitHub:
		return "github"
	case FocusSummary:
		return "summary"
	default:
		return "unknown"
	}
}

// FocusChangedMsg is sent when focus changes between panels.
type FocusChangedMsg struct {
	Panel FocusPanel
}
