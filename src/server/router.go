package server

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/derkajecht/Beatrice/src/shared"
)

// HandleHandshake receives the handshake packet from the client
// validates username and public key, and saves the client to the hub
// returns an error if the handshake fails
func HandleHandshake(h *Hub, c *ServerClient, p shared.GeneralPacket) error {
	var hs shared.HandshakePacket
	if err := json.Unmarshal(p.Message, &hs); err != nil {
		return fmt.Errorf("malformed handshake: %w", err)
	}

	// Nickname validation
	nickname := strings.TrimSpace(UsernameSanitizer(hs.Nickname))
	if len(nickname) < 3 {
		return fmt.Errorf("nickname too short")
	}

	// Store user in the db
	if err := h.db.StoreUser(nickname, hs.PubKey); err != nil {
		return fmt.Errorf("failed to store user: %w", err)
	}

	// Store nickname and pub key in memory
	c.Nickname = nickname
	c.PubKey = hs.PubKey

	// Init dir packet
	dirPacket := shared.DirPacket{
		CurrentUsers: make(map[string][]byte),
	}

	// Add clients to dir packet
	h.mu.RLock()
	for _, client := range h.clients {
		dirPacket.CurrentUsers[client.Nickname] = client.PubKey
	}
	h.mu.RUnlock()

	// Send dir packet to client
	if err := SendPacketToClient(c, "d", dirPacket); err != nil {
		return fmt.Errorf("failed to send directory: %w", err)
	}

	// Broadcast join event to all clients
	h.Broadcast("j", shared.JoinPacket{Nickname: nickname, PubKey: hs.PubKey})

	return nil
}

// HandleMessage handles incoming message packets
func HandleMessage(h *Hub, c *ServerClient, p shared.GeneralPacket) error {
	var mp shared.MessagePacket
	if err := json.Unmarshal(p.Message, &mp); err != nil {
		return fmt.Errorf("malformed message: %w", err)
	}
	// reject messages without a sender - the sender must come from the client
	// itself, never substitute the connection's nickname
	if mp.Sender == "" {
		return fmt.Errorf("message without sender rejected")
	}
	h.Broadcast("m", mp)
	return nil
}

// HandleJoin handles join requests
func HandleJoin(h *Hub, c *ServerClient, p shared.GeneralPacket) error {
	var jp shared.JoinPacket
	if err := json.Unmarshal(p.Message, &jp); err != nil {
		return fmt.Errorf("malformed join: %w", err)
	}
	h.Broadcast("j", jp)
	return nil
}

// HandleLeave handles leave requests
func HandleLeave(h *Hub, c *ServerClient, p shared.GeneralPacket) error {
	var lp shared.LeavePacket
	if err := json.Unmarshal(p.Message, &lp); err != nil {
		return fmt.Errorf("malformed leave: %w", err)
	}
	h.Broadcast("l", lp)
	return nil
}
