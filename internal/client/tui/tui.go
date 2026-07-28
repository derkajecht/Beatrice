package tui

func NewTUI(logCh <-chan []byte) {
	// read from the logCh channel to handle the TUI
	msg := <-logCh
}
