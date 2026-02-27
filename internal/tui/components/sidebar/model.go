package sidebar

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/edalee/koko/internal/tui/components/github"
	"github.com/edalee/koko/internal/tui/components/slack"
	"github.com/edalee/koko/internal/tui/components/summary"
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
	return lipgloss.JoinVertical(lipgloss.Left,
		m.Slack.View(),
		m.GitHub.View(),
		m.Summary.View(),
	)
}

func (m Model) SetSize(width, height int) Model {
	m.width = width
	m.height = height

	slackH := height / 3
	githubH := height / 3
	summaryH := height - slackH - githubH

	m.Slack = m.Slack.SetSize(width, slackH)
	m.GitHub = m.GitHub.SetSize(width, githubH)
	m.Summary = m.Summary.SetSize(width, summaryH)

	return m
}
