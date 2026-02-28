package terminal

import (
	"os"
	"os/exec"
	"sync"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/vt"
	"github.com/creack/pty"
)

// ptyState holds the shared mutable state for the embedded terminal.
// It lives on the heap behind a pointer so Bubble Tea's value-copy
// semantics don't break file descriptors or goroutines.
type ptyState struct {
	ptmx     *os.File
	cmd      *exec.Cmd
	vt       *vt.Emulator
	mu       sync.Mutex   // protects vt (goroutine writes, View reads)
	outputCh chan struct{} // signals new PTY output (buffered, 1)
	exitCh   chan error    // signals shell exit (buffered, 1)
	started  bool
	exited   bool
	exitSub  bool // true if waitForExit subscription is active
}

// startShell is a tea.Cmd that spawns zsh in a PTY with the given size
// and starts the read loop goroutine. Returns an OutputMsg on success
// to kick off the subscription chain.
func startShell(state *ptyState, cols, rows int) tea.Cmd {
	return func() tea.Msg {
		state.vt = vt.NewEmulator(cols, rows)

		cmd := exec.Command("zsh", "-l")
		cmd.Env = append(os.Environ(),
			"TERM=xterm-256color",
			"COLORTERM=truecolor",
		)

		ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{
			Rows: uint16(rows),
			Cols: uint16(cols),
		})
		if err != nil {
			return ExitedMsg{Err: err}
		}

		state.ptmx = ptmx
		state.cmd = cmd
		state.started = true

		go state.readLoop()
		go state.responseLoop()

		return OutputMsg{}
	}
}

// readLoop continuously reads from the PTY master and feeds bytes into
// the VT emulator. Signals outputCh (coalescing) on each read, and
// sends the final error to exitCh when the PTY closes.
func (s *ptyState) readLoop() {
	buf := make([]byte, 32*1024)
	for {
		n, err := s.ptmx.Read(buf)
		if n > 0 {
			s.mu.Lock()
			s.vt.Write(buf[:n])
			s.mu.Unlock()
			// Non-blocking signal — coalesces multiple writes into one.
			select {
			case s.outputCh <- struct{}{}:
			default:
			}
		}
		if err != nil {
			s.exitCh <- err
			return
		}
	}
}

// responseLoop drains the VT emulator's internal pipe and writes
// responses back to the PTY master. The VT emulator writes to its
// pipe when the shell queries terminal capabilities (device attributes,
// cursor position, etc.). Without this loop, those writes block on
// the zero-buffered io.Pipe and deadlock the readLoop.
func (s *ptyState) responseLoop() {
	buf := make([]byte, 4096)
	for {
		n, err := s.vt.Read(buf)
		if n > 0 {
			s.ptmx.Write(buf[:n])
		}
		if err != nil {
			return
		}
	}
}

// waitForOutput returns a tea.Cmd that blocks until the read loop
// signals new output, then returns an OutputMsg.
func (s *ptyState) waitForOutput() tea.Cmd {
	return func() tea.Msg {
		_, ok := <-s.outputCh
		if !ok {
			return nil
		}
		return OutputMsg{}
	}
}

// waitForExit returns a tea.Cmd that blocks until the shell process
// exits, then returns an ExitedMsg.
func (s *ptyState) waitForExit() tea.Cmd {
	return func() tea.Msg {
		err, ok := <-s.exitCh
		if !ok {
			return nil
		}
		return ExitedMsg{Err: err}
	}
}

// Close tears down the PTY and waits for the shell to exit.
func (s *ptyState) Close() {
	if s == nil || !s.started {
		return
	}
	s.vt.Close()
	s.ptmx.Close()
	// Don't s.cmd.Wait() synchronously — the readLoop and responseLoop
	// goroutines will exit when ptmx closes, and the process will be
	// reaped by the OS.
}
