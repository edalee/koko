package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/edalee/koko/internal/tui/components/sidebar"
	"github.com/edalee/koko/internal/tui/components/terminal"
)

const sidebarWidth = 30

type Model struct {
	terminal       terminal.Model
	sidebar        sidebar.Model
	focus          FocusPanel
	width          int
	height         int
	sidebarVisible bool
}

func New() Model {
	return Model{
		terminal:       terminal.New(),
		sidebar:        sidebar.New(),
		focus:          FocusTerminal,
		sidebarVisible: true,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m = m.updateSizes()
		m = m.updateFocus()
		return m, nil

	case tea.KeyPressMsg:
		switch msg.String() {
		case KeyQuit:
			return m, tea.Quit
		case KeyToggleSidebar:
			m.sidebarVisible = !m.sidebarVisible
			if !m.sidebarVisible && m.focus != FocusTerminal {
				m.focus = FocusTerminal
			}
			m = m.updateFocus()
			m = m.updateSizes()
			return m, nil
		case KeyFocusTerminal:
			m.focus = FocusTerminal
			m = m.updateFocus()
			return m, nil
		case KeyFocusSlack:
			if !m.sidebarVisible {
				m.sidebarVisible = true
			}
			m.focus = FocusSlack
			m = m.updateFocus()
			m = m.updateSizes()
			return m, nil
		case KeyFocusGitHub:
			if !m.sidebarVisible {
				m.sidebarVisible = true
			}
			m.focus = FocusGitHub
			m = m.updateFocus()
			m = m.updateSizes()
			return m, nil
		}
	}

	var cmd tea.Cmd
	switch m.focus {
	case FocusTerminal:
		m.terminal, cmd = m.terminal.Update(msg)
	default:
		m.sidebar, cmd = m.sidebar.Update(msg)
	}

	return m, cmd
}

func (m Model) View() tea.View {
	if m.width == 0 || m.height == 0 {
		return tea.NewView("")
	}

	contentHeight := max(m.height-1, 0)

	var content string
	if m.sidebarVisible {
		content = lipgloss.JoinHorizontal(lipgloss.Top,
			m.terminal.View(),
			m.sidebar.View(),
		)
	} else {
		content = m.terminal.View()
	}

	content = lipgloss.NewStyle().
		Width(m.width).
		Height(contentHeight).
		Render(content)

	statusBar := m.renderStatusBar()
	v := tea.NewView(lipgloss.JoinVertical(lipgloss.Left, content, statusBar))
	v.AltScreen = true
	return v
}

func (m Model) renderStatusBar() string {
	bindings := []string{
		"ctrl+t: terminal",
		"ctrl+s: slack",
		"ctrl+g: github",
		"ctrl+\\: sidebar",
		"ctrl+c: quit",
	}
	bar := " " + strings.Join(bindings, "  ")
	return StatusBarStyle.Width(m.width).Render(bar)
}

func (m Model) updateSizes() Model {
	contentHeight := max(m.height-1, 0)

	if m.sidebarVisible {
		termWidth := max(m.width-sidebarWidth, 0)
		m.terminal = m.terminal.SetSize(termWidth, contentHeight)
		m.sidebar = m.sidebar.SetSize(sidebarWidth, contentHeight)
	} else {
		m.terminal = m.terminal.SetSize(m.width, contentHeight)
	}

	return m
}

func (m Model) updateFocus() Model {
	m.terminal = m.terminal.SetFocus(m.focus == FocusTerminal)
	m.sidebar.Slack = m.sidebar.Slack.SetFocus(m.focus == FocusSlack)
	m.sidebar.GitHub = m.sidebar.GitHub.SetFocus(m.focus == FocusGitHub)
	m.sidebar.Summary = m.sidebar.Summary.SetFocus(m.focus == FocusSummary)
	return m
}
