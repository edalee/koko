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

	contentW := max(m.width-2, 0) // content width inside border
	sep := lipgloss.NewStyle().Foreground(borderColor).
		Render(strings.Repeat("─", contentW))

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
		Width(m.width).
		Height(m.height).
		Render(inner)
}

func (m Model) SetSize(width, height int) Model {
	m.width = width
	m.height = height

	contentW := max(width-2, 0)  // inside border
	contentH := max(height-2, 0) // inside border
	panelH := max(contentH-2, 0) // minus 2 separator lines

	slackH := panelH / 3
	githubH := panelH / 3
	summaryH := panelH - slackH - githubH

	m.Slack = m.Slack.SetSize(contentW, slackH)
	m.GitHub = m.GitHub.SetSize(contentW, githubH)
	m.Summary = m.Summary.SetSize(contentW, summaryH)

	return m
}
