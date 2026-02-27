package github

import (
	"fmt"
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
	return Model{}
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

	title := lipgloss.NewStyle().Bold(true).Foreground(titleColor).Render(" GitHub")
	dim := lipgloss.NewStyle().Foreground(dimColor)

	lines := []string{
		title,
		fmt.Sprintf("  drumstick2   %s", dim.Render("2 PRs")),
		fmt.Sprintf("  drumstick-ui %s", dim.Render("1 PR")),
		fmt.Sprintf("  trigon       %s", dim.Render("0 PRs")),
	}

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
