package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/coder/websocket"
	"github.com/derkajecht/Beatrice/internal/shared"
)

// SendPacketToClient sends a packet or message to the client
// and returns an error if any
func (h *Hub) SendPacketToClient(c *ServerClient, packetType string, innerPacket any) error {

	ctx := context.Background()
	writeCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// get the client from the hub
	client := h.getClient(c.ID)

	// marshal inner packet to JSON
	innerBytes, marshalErr := json.Marshal(innerPacket)
	if marshalErr != nil {
		slog.Error("err_marshalling_inner_packet", "err", marshalErr, "client", client)
		return marshalErr
	}

	// create envelope struct and wrap inner packet in it
	envelope := shared.GeneralPacket{
		Type:    packetType,
		Message: innerBytes,
	}

	envelopeBytes, marshalErr := json.Marshal(envelope)
	if marshalErr != nil {
		slog.Error("err_marshalling_envelope", "err", marshalErr, "client", client)
		return marshalErr
	}

	// marshal envelope struct to JSON
	if err := client.Conn.Write(writeCtx, websocket.MessageText, envelopeBytes); err != nil {
		slog.Error("err_marshalling_envelope", "err", err)
		return err
	}

	return nil
}

// SendMessage sends a message to the server when the user presses enter in the TUI
// func (u User) SendMessage(ctx context.Context, msg []byte) error {
// 	writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
// 	defer cancel()
// 	return u.Conn.Write(writeCtx, websocket.MessageText, msg)
// }

// DisconnectAndQuit closes the connection and quits the application
// func DisconnectAndQuit(conn net.Conn) {
// 	SendPacketToClient(conn, "q", &ErrPacket{
// 		Message: "err_connection_closed",
// 	})
// 	conn.Close()
// }
