package sidebar

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/edalee/koko/internal/tui/components/github"
	"github.com/edalee/koko/internal/tui/components/slack"
	"github.com/edalee/koko/internal/tui/components/summary"
)

var (
	activeBorderColor   = lipgloss.Color("62")
	inactiveBorderColor = lipgloss.Color("240")
)

type Model struct {
	Slack   slack.Model
	GitHub  github.Model
	Summary summary.Model
	width   int
	height  int
}

func New() Model {
	return Model{
		Slack:   slack.New(),
		GitHub:  github.New(),
		Summary: summary.New(),
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	return m, nil
}

func (m Model) View() string {
	borderColor := inactiveBorderColor
	if m.Slack.Focused() || m.GitHub.Focused() || m.Summary.Focused() {
		borderColor = activeBorderColor
	}

	innerW := max(m.width-2, 0)
	sep := lipgloss.NewStyle().Foreground(borderColor).
		Render(strings.Repeat("─", innerW))

	inner := lipgloss.JoinVertical(lipgloss.Left,
		m.Slack.View(),
		sep,
		m.GitHub.View(),
		sep,
		m.Summary.View(),
	)

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(innerW).
		Height(max(m.height-2, 0)).
		Render(inner)
}

func (m Model) SetSize(width, height int) Model {
	m.width = width
	m.height = height

	innerW := max(width-2, 0)
	innerH := max(height-2, 0)
	contentH := max(innerH-2, 0) // subtract 2 separator lines

	slackH := contentH / 3
	githubH := contentH / 3
	summaryH := contentH - slackH - githubH

	m.Slack = m.Slack.SetSize(innerW, slackH)
	m.GitHub = m.GitHub.SetSize(innerW, githubH)
	m.Summary = m.Summary.SetSize(innerW, summaryH)

	return m
}
