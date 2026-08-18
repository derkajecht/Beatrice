package server

import (
	"bytes"
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

	storedPubKey, exists, err := h.db.GetUser(nickname)
	if err != nil {
		return fmt.Errorf("failed to look up user: %w", err)
	}

	c.Nickname = nickname
	c.PubKey = hs.PubKey

	// New user: trust on first use — register and complete the handshake.
	if !exists {
		if err := h.db.StoreUser(nickname, hs.PubKey); err != nil {
			return fmt.Errorf("failed to store user: %w", err)
		}
		c.Verified = true
		return completeHandshake(h, c) // sends dir packet to user and broadcasts join packet to all other active users
	}

	// Returning user: must prove ownership of the stored public key.
	if !bytes.Equal(storedPubKey, hs.PubKey) {
		return fmt.Errorf("public key mismatch for existing user %q", nickname)
	}

	nonce, err := h.nonces.Issue()
	if err != nil {
		return fmt.Errorf("failed to issue challenge nonce: %w", err)
	}

	challenge := shared.ChallengePacket{
		Nickname:    nickname,
		PendingAuth: nonce,
		PubKey:      storedPubKey,
		IsNew:       false,
	}
	if err := SendPacketToClient(c, "c", challenge); err != nil {
		return fmt.Errorf("failed to send challenge: %w", err)
	}
	return nil // handshake pending until the challenge response is verified
}

func HandleChallenge(h *Hub, c *ServerClient, p shared.GeneralPacket) error {
	var cr shared.ChallengeResponse
	if err := json.Unmarshal(p.Message, &cr); err != nil {
		return fmt.Errorf("malformed challenge: %w", err)
	}
	// check signature matches the challenge nonce
	// if successful, mark user as verified and complete handshake
	if !CheckChallenge(&cr, c, h) {
		return fmt.Errorf("challenge verification failed")
	}
	c.Verified = true
	return completeHandshake(h, c)
}

// completeHandshake finishes a successful handshake: sends the directory packet
// and broadcasts the join event. Shared by new users and verified returning users.
func completeHandshake(h *Hub, c *ServerClient) error {
	dirPacket := shared.DirPacket{
		CurrentUsers: make(map[string][]byte),
	}

	h.mu.RLock()
	for _, client := range h.clients {
		dirPacket.CurrentUsers[client.Nickname] = client.PubKey
	}
	h.mu.RUnlock()

	if err := SendPacketToClient(c, "d", dirPacket); err != nil {
		return fmt.Errorf("failed to send directory: %w", err)
	}
	h.Broadcast("j", shared.JoinPacket{Nickname: c.Nickname, PubKey: c.PubKey})
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
