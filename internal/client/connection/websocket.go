package websocket

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"time"

	"github.com/coder/websocket"
	"github.com/derkajecht/Beatrice/internal/client/crypto"
	"github.com/derkajecht/Beatrice/internal/shared/sharedvalidation"
	"github.com/derkajecht/Beatrice/internal/shared/types"
)

type LocalClient struct {
	types.Client
}

// NewChatClient establishes a connection to the server using the host and port provided.
func NewChatClient(ctx context.Context, addr string) (*types.Client, error) {

	// Open client connection to the server using the host and port provided
	maxDelay := 30 * time.Second
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
				return LocalClient{Client: types.Client{}}, ctx.Err()
			}
			delay = min(delay*2, maxDelay)
			continue
		}
		delay = time.Second

		// create a new client instance, pass the connection and a channel for tui messages
		client := types.Client{
			Conn:    conn,
			TuiChan: make(chan []byte, 100),
		}

		// run readloop in a goroutine for async reading
		go client.readLoop(ctx)

	}
}

func (c LocalClient) readLoop(ctx context.Context) {
	defer c.Conn.CloseNow()

	for {
		_, msg, err := c.Conn.Read(ctx)
		if err != nil {
			slog.Error("Error reading from websocket connection", "err", err)
			return
		}

		// send msg to the hub
		c.TuiChan <- msg
	}
}

// SendMessage sends a message to the server when the user presses enter in the TUI
func (c LocalClient) SendMessage(ctx context.Context, msg []byte) error {
	writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return c.Conn.Write(writeCtx, websocket.MessageText, msg)
}

// StartClient establishes a connection to the server using the host and port provided.
// It returns an error if the host or port is empty.
func StartClient(host, port string) {

	if sharedvalidation.HasEmptyArgs(host, port) {
		slog.Warn("No host or port provided: Defaulting to localhost:8080")
	}

	// Call crypto suite to generate a new key pair
	// stores the public and private keys in the user session
	_, _, _, err := crypto.NewUserSession()
	if err != nil {
		println("Error generating user session:", err)
		return
	}

	// format the address string
	addr := fmt.Sprintf("%s:%s", host, port)
	// create a new context with a timeout of 30 seconds
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// connect with backoff
	NewChatClient(ctx, addr)

	// TODO: call to start the tui
}
