package client

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/derkajecht/Beatrice/internal/shared"
)

// NewChatClient establishes a connection to the server using the host and port provided.
func (u *User) NewChatClient(ctx context.Context, addr string) error {

	for {
		conn, _, err := websocket.Dial(ctx, addr, nil)
		if err != nil {
			slog.Error("Error connecting to server", "err", err)
			return err
		}

		// create a new client instance, pass the connection and a channel for tui messages
		u.Conn = conn
		defer u.Conn.Close(websocket.StatusInternalError, "Client closed")

		// run readloop in a goroutine for async reading
		if err := u.readLoop(ctx); err != nil {
			slog.Error("Error reading from websocket connection", "err", err)
			return err
		}
	}
}

func (u *User) readLoop(ctx context.Context) error {
	defer u.Conn.CloseNow()

	for {
		_, msg, err := u.Conn.Read(ctx) // msg is []byte
		if err != nil {
			slog.Error("Error reading from websocket connection", "err", err)
			return fmt.Errorf("error reading from websocket connection: %w", err)
		}

		// unmarshal the message to json
		var packet shared.GeneralPacket
		err = json.Unmarshal(msg, &packet)
		if err != nil {
			slog.Error("Error unmarshalling message packet", "err", err)
			break
		}
		slog.Info("packet successfully unmarshalled", "packet", packet)

		// send msg packet to the hub
		u.TuiChan <- packet
	}
	return nil
}

// SendPacketToClient sends a packet or message to the client
// and returns an error if any
func (u *User) SendPacketToServer(packetType string, innerPacket any) error {

	ctx := context.Background()
	writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// marshal inner packet to JSON
	innerBytes, marshalErr := json.Marshal(innerPacket)
	if marshalErr != nil {
		slog.Error("err_marshalling_inner_packet", "err", marshalErr)
		return marshalErr
	}

	// create envelope struct and wrap inner packet in it
	envelope := shared.GeneralPacket{
		Type:    packetType,
		Message: innerBytes,
	}

	// marshal envelope struct to JSON
	if err := wsjson.Write(ctx, u.Conn, envelope); err != nil {
		slog.Error("err_marshalling_envelope", "err", err)
		return err
	}
	return nil
}

// StartClient establishes a connection to the server using the host and port provided.
// It returns an error if the host or port is empty.
func StartClient(host, port string) error {

	if shared.HasEmptyArgs(host, port) {
		slog.Warn("No host or port provided: Defaulting to localhost:8080")
		host = "localhost"
		port = "8080"
	}

	// Call crypto suite to generate a new key pair
	// stores the public and private keys in the user session
	_, _, err := NewUserSession()
	if err != nil {
		slog.Error("Error generating user session", "err", err)
		return
	}

	// format the address string
	addr := fmt.Sprintf("ws://%s:%s", host, port)

	// create a new context with a timeout of 30 seconds
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// create a new user instance
	// initiates the conn and tuichan fields
	user := NewUser()

	err = user.NewChatClient(ctx, addr)
	if err != nil {
		slog.Error("Error connecting to server", "err", err)
		return err
	}
	slog.Info("Connected to server", "connected", true, "addr", addr)
	// close the connection when the TUI exits
	// 1000 is the close code for normal closure
	defer user.Conn.Close(websocket.StatusNormalClosure, "")
	slog.Info("Closing connection", "connected", false) // slog message to inform the tui that connection is closed
}
