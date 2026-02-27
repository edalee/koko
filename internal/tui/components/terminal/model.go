package terminal

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

var (
	activeBorderColor   = lipgloss.Color("62")
	inactiveBorderColor = lipgloss.Color("240")
	titleColor          = lipgloss.Color("62")
	dimColor            = lipgloss.Color("240")
)

type Model struct {
	width   int
	height  int
	focused bool
}

func New() Model {
	return Model{focused: true}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	return m, nil
}

func (m Model) View() string {
	w := max(m.width-2, 0)
	h := max(m.height-2, 0)

	borderColor := inactiveBorderColor
	if m.focused {
		borderColor = activeBorderColor
	}

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(w).
		Height(h)

	title := lipgloss.NewStyle().Bold(true).Foreground(titleColor).Render(" Terminal")
	hint := lipgloss.NewStyle().Foreground(dimColor).Render("  (placeholder)")
	toggle := lipgloss.NewStyle().Foreground(dimColor).Render("  Press ctrl+\\ to toggle sidebar")

	lines := []string{title, "", hint, "", toggle}
	return style.Render(strings.Join(lines, "\n"))
}

func (m Model) SetSize(width, height int) Model {
	m.width = width
	m.height = height
	return m
}

func (m Model) SetFocus(focused bool) Model {
	m.focused = focused
	return m
}
