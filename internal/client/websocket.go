package client

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"time"

	"github.com/coder/websocket"
	"github.com/derkajecht/Beatrice/internal/client/tui"
	"github.com/derkajecht/Beatrice/internal/shared"
)

// NewChatClient establishes a connection to the server using the host and port provided.
func NewChatClient(ctx context.Context, addr string) (User, error) {

	// Open client connection to the server using the host and port provided
	maxDelay := 10 * time.Second
	delay := time.Second

	for {
		conn, _, err := websocket.Dial(ctx, addr, nil)
		if err != nil {
			jitter := time.Duration(rand.Int63n(int64(delay / 2)))
			wait := delay + jitter
			slog.Error("Error connecting to server, retrying in %v", "err", err, "wait", wait)
			select {
			case <-time.After(wait):
			case <-ctx.Done():
				return User{}, ctx.Err()
			}
			delay = min(delay*2, maxDelay)
			continue
		}
		delay = time.Second

		// create a new client instance, pass the connection and a channel for tui messages
		client := User{
			Conn:    conn,
			TuiChan: make(chan []byte, 100),
		}

		// run readloop in a goroutine for async reading
		if err := client.readLoop(ctx); err != nil {
			slog.Error("Error reading from websocket connection", "err", err)
			return User{}, err
		}

		// NOTE: not sure if this is needed
		defer client.Conn.Close(websocket.StatusInternalError, "Client closed")
		return client, nil
	}
}

func (u User) readLoop(ctx context.Context) error {
	defer u.Conn.CloseNow()

	for {
		_, msg, err := u.Conn.Read(ctx)
		if err != nil {
			slog.Error("Error reading from websocket connection", "err", err)
			return fmt.Errorf("error reading from websocket connection: %w", err)
		}

		// send msg to the hub
		u.TuiChan <- msg
	}
}

// SendMessage sends a message to the server when the user presses enter in the TUI
func (u User) SendMessage(ctx context.Context, msg []byte) error {
	writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return u.Conn.Write(writeCtx, websocket.MessageText, msg)
}

// StartClient establishes a connection to the server using the host and port provided.
// It returns an error if the host or port is empty.
func StartClient(host, port string) {

	if shared.HasEmptyArgs(host, port) {
		slog.Warn("No host or port provided: Defaulting to localhost:8080")
		host = "localhost"
		port = "8080"
	}

	// setup the logger and read-only channel for the TUI
	// start the TUI in a goroutine - non-blocking
	logCh := LoggerSetup()
	go tui.NewTUI(logCh)

	// Call crypto suite to generate a new key pair
	// stores the public and private keys in the user session
	_, _, err := NewUserSession()
	if err != nil {
		slog.Error("Error generating user session", "err", err)
		return
	}

	// format the address string
	addr := fmt.Sprintf("%s:%s", host, port)

	// create a new context with a timeout of 30 seconds
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	chatClient, err := NewChatClient(ctx, addr)
	if err != nil {
		slog.Error("Error connecting to server", "err", err)
		return
	}
	slog.Info("Connected to server", "connected", true, "addr", addr)
	// close the connection when the TUI exits
	// 1000 is the close code for normal closure
	defer chatClient.Conn.Close(1000, "Goodbye")
	slog.Info("Closing connection", "connected", false) // slog message to inform the tui that connection is closed

	// TODO: call to start the tui
}
