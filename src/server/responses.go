package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/coder/websocket"
	"github.com/derkajecht/Beatrice/src/shared"
)

// SendPacketToClient sends a packet or message to the client
// and returns an error if any
func SendPacketToClient(c *ServerClient, packetType string, innerPacket any) error {
	if c == nil || c.Conn == nil {
		return fmt.Errorf("client not registered")
	}

	writeCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// marshal inner packet to JSON
	innerBytes, marshalErr := json.Marshal(innerPacket)
	if marshalErr != nil {
		return fmt.Errorf("err_marshalling_inner_packet: %w", marshalErr)
	}

	// create envelope struct and wrap inner packet in it
	envelope := shared.GeneralPacket{
		Type:    packetType,
		Message: innerBytes,
	}

	envelopeBytes, marshalErr := json.Marshal(envelope)
	if marshalErr != nil {
		return fmt.Errorf("err_marshalling_envelope: %w", marshalErr)
	}

	// write directly to the passed client's connection
	if err := c.Conn.Write(writeCtx, websocket.MessageText, envelopeBytes); err != nil {
		return fmt.Errorf("err_writing_to_client: %w", err)
	}

	return nil
}

// Broadcast sends a packet to all connected clients
func (h *Hub) Broadcast(packetType string, innerPacket any) {
	h.mu.RLock()
	// TODO: Do not hold h.mu.RLock while performing network writes. A slow
	// client can hold this lock for the 30-second write timeout and block
	// addClient/removeClient for every connection.
	defer h.mu.RUnlock()
	for _, c := range h.clients {
		if err := SendPacketToClient(c, packetType, innerPacket); err != nil {
			slog.Error("error broadcasting to client", "client", c.ID, "err", err)
		}
	}
}
