package terminal

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/creack/pty"
)

var (
	activeBorderColor   = lipgloss.Color("62")
	inactiveBorderColor = lipgloss.Color("240")
)

const (
	defaultCols = 80
	defaultRows = 24
)

type Model struct {
	width   int
	height  int
	focused bool
	state   *ptyState // shared across value copies
}

func New() Model {
	return Model{
		focused: true,
		state: &ptyState{
			outputCh: make(chan struct{}, 1),
			exitCh:   make(chan error, 1),
		},
	}
}

func (m Model) Init() tea.Cmd {
	return startShell(m.state, defaultCols, defaultRows)
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if m.focused && m.state != nil && m.state.started && !m.state.exited {
			if b := keyToBytes(msg); len(b) > 0 {
				m.state.ptmx.Write([]byte(b))
			}
		}
		return m, nil

	case OutputMsg:
		cmds := []tea.Cmd{m.state.waitForOutput()}
		if !m.state.exitSub {
			m.state.exitSub = true
			cmds = append(cmds, m.state.waitForExit())
		}
		return m, tea.Batch(cmds...)

	case ExitedMsg:
		m.state.exited = true
		return m, nil
	}

	return m, nil
}

func (m Model) View() string {
	borderColor := inactiveBorderColor
	if m.focused {
		borderColor = activeBorderColor
	}

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(m.width).
		Height(m.height)

	var content string
	if m.state != nil && m.state.started {
		m.state.mu.Lock()
		content = m.state.vt.Render()
		m.state.mu.Unlock()
	}

	return style.Render(content)
}

func (m Model) SetSize(width, height int) Model {
	m.width = width
	m.height = height

	// Inner size after border (1 cell each side)
	innerW := max(width-2, 1)
	innerH := max(height-2, 1)

	if m.state != nil && m.state.started && !m.state.exited {
		m.state.mu.Lock()
		m.state.vt.Resize(innerW, innerH)
		m.state.mu.Unlock()
		pty.Setsize(m.state.ptmx, &pty.Winsize{
			Rows: uint16(innerH),
			Cols: uint16(innerW),
		})
	}

	return m
}

func (m Model) SetFocus(focused bool) Model {
	m.focused = focused
	return m
}

func (m Model) Close() {
	m.state.Close()
}
