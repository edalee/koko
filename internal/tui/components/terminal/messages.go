package terminal

// OutputMsg signals that the VT emulator has new content to render.
type OutputMsg struct{}

// ExitedMsg signals that the shell process has exited.
type ExitedMsg struct {
	Err error
}
