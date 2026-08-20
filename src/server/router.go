package server

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/derkajecht/Beatrice/src/shared"
)

func shortHash(pk []byte) string {
	hasher := sha256.New()
	hasher.Write(pk)
	hash := hex.EncodeToString(hasher.Sum(nil))
	return hash
}

func (c *ServerClient) SendChallenge(h *Hub, nonce string) error {
	challenge := shared.ChallengePacket{
		Nickname:    c.Nickname,
		PendingAuth: nonce,
		PubKey:      c.PubKey,
		IsNew:       false,
	}
	if err := SendPacketToClient(c, "c", challenge); err != nil {
		return fmt.Errorf("failed to send challenge: %w", err)
	}
	return nil
}

// HandleHandshake receives the handshake packet from the client
// validates username and public key, and saves the client to the hub
// returns an error if the handshake fails
func HandleHandshake(h *Hub, c *ServerClient, p shared.GeneralPacket) error {
	var hs shared.HandshakePacket
	if err := json.Unmarshal(p.Message, &hs); err != nil {
		return fmt.Errorf("malformed handshake: %w", err)
	}

	// Nickname validation and duplicate check
	nickname := strings.TrimSpace(UsernameSanitizer(hs.Nickname))
	if len(nickname) < 3 {
		return fmt.Errorf("nickname too short")
	}

	storedPubKey, exists, err := h.db.GetUserPK(nickname)
	if err != nil {
		return fmt.Errorf("failed to look up user: %w", err)
	}

	c.Nickname = nickname
	c.PubKey = hs.PubKey

	if !exists {
		// TOFU register
		if err := h.db.StoreUser(nickname, hs.PubKey); err != nil {
			return fmt.Errorf("failed to store user: %w", err)
		}
		c.Verified = true
		return completeHandshake(h, c) // sends dir packet to user and broadcasts join packet to all other active users
	}

	// returning user
	if bytes.Equal(storedPubKey, hs.PubKey) {
		nonce, err := h.nonces.Issue()
		// challenge flow
		if err != nil {
			return fmt.Errorf("failed to issue challenge nonce: %w", err)
		}
		if err := c.SendChallenge(h, nonce); err != nil {
			return fmt.Errorf("failed to send challenge: %w", err)
		}
		return nil
	}

	cand := nickname + "-" + shortHash(hs.PubKey)
	cpk, cexists, err := h.db.GetUserPK(cand)
	if err != nil {
		return fmt.Errorf("failed to look up user: %w", err)
	}

	slog.Info("assigned suffix nickname", "original", nickname, "assigned", cand)
	if cexists && !bytes.Equal(cpk, hs.PubKey) {
		return fmt.Errorf("hash collision on %q", cand) // ~impossible
	}

	c.Nickname = cand
	if err := SendPacketToClient(c, "n", shared.NicknameUpdatePacket{Nickname: cand}); err != nil {
		return fmt.Errorf("failed to send nickname change packet: %w", err)
	}

	if !cexists {
		// New user under suffixed name -> TOFU register and complete
		if err := h.db.StoreUser(cand, hs.PubKey); err != nil {
			return fmt.Errorf("failed to store user: %w", err)
		}
		c.Verified = true
		return completeHandshake(h, c)
	}
	// Returning user under suffixed name -> prove ownership
	nonce, err := h.nonces.Issue()
	if err != nil {
		return fmt.Errorf("failed to issue challenge nonce: %w", err)
	}
	return c.SendChallenge(h, nonce)
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
