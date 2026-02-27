package slack

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

var (
	titleColor    = lipgloss.Color("62")
	dimTitleColor = lipgloss.Color("240")
	dimColor      = lipgloss.Color("240")
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
	tc := dimTitleColor
	if m.focused {
		tc = titleColor
	}

	title := lipgloss.NewStyle().Bold(true).Foreground(tc).Render(" Slack")
	dim := lipgloss.NewStyle().Foreground(dimColor)

	lines := []string{
		title,
		fmt.Sprintf("  DMs:      %s", dim.Render("3 unread")),
		fmt.Sprintf("  Threads:  %s", dim.Render("1 reply")),
		fmt.Sprintf("  Mentions: %s", dim.Render("2 new")),
	}

	return lipgloss.NewStyle().
		Width(m.width).
		Height(m.height).
		Render(strings.Join(lines, "\n"))
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

func (m Model) Focused() bool {
	return m.focused
}
