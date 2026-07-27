package logger

import "log/slog"

// ChannelWriter implements io.Writer for slog
type ChannelWriter struct {
	Ch chan<- []byte
}

// helper function that wraps the channel to implement io.Writer
func (w *ChannelWriter) Write(p []byte) (int, error) {
	// Must copy because slog reuses this memory buffer internally
	buf := make([]byte, len(p))
	copy(buf, p)

	// send down the logCh channel for the TUI to read
	w.Ch <- buf

	// return the number of bytes written
	return len(p), nil
}

// LoggerSetup initializes slog and returns the read-only channel for your TUI
func LoggerSetup() {
	// Create a buffered channel so logging won't instantly block
	logCh := make(chan []byte, 100)

	// Create the handler and default logger
	slog.SetDefault(slog.New(slog.NewJSONHandler(&ChannelWriter{Ch: logCh}, nil)))
}
