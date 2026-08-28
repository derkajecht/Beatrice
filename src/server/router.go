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
	c.HPKEPubKey = hs.HPKEPubKey

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
// The directory and join packets carry HPKE public keys (not ed25519 identity
// keys) so peers can encrypt messages to each other.
func completeHandshake(h *Hub, c *ServerClient) error {
	dirPacket := shared.DirPacket{
		CurrentUsers: make(map[string][]byte),
	}

	h.mu.RLock()
	for _, client := range h.clients {
		dirPacket.CurrentUsers[client.Nickname] = client.HPKEPubKey
	}
	h.mu.RUnlock()

	if err := SendPacketToClient(c, "d", dirPacket); err != nil {
		return fmt.Errorf("failed to send directory: %w", err)
	}
	h.Broadcast("j", shared.JoinPacket{Nickname: c.Nickname, PubKey: c.HPKEPubKey})
	return nil
}

// HandleMessage handles incoming message packets. Messages are encrypted
// per recipient by the sender, so they are routed unicast to the named
// recipient only — never broadcast. The server cannot decrypt them. Both the
// sender and the target must be verified connections; the wire sender field is
// overwritten with the connection's authenticated nickname.
func HandleMessage(h *Hub, c *ServerClient, p shared.GeneralPacket) error {
	var mp shared.MessagePacket
	if err := json.Unmarshal(p.Message, &mp); err != nil {
		return fmt.Errorf("malformed message: %w", err)
	}
	// only authenticated clients may send messages
	if !c.Verified || c.Nickname == "" {
		slog.Warn("message from unverified connection rejected", "client", c.ID)
		if err := SendPacketToClient(c, "e", &shared.ErrPacket{Message: "err_unverified_sender"}); err != nil {
			return fmt.Errorf("failed to send unverified-sender error: %w", err)
		}
		return nil
	}

	// unicast requires an explicit recipient
	if mp.Recipient == "" {
		slog.Warn("message without recipient rejected", "sender", c.Nickname)
		if err := SendPacketToClient(c, "e", &shared.ErrPacket{Message: "err_missing_recipient"}); err != nil {
			return fmt.Errorf("failed to send missing-recipient error: %w", err)
		}
		return nil
	}

	target := h.getClientByNickname(mp.Recipient)
	if target == nil || !target.Verified {
		slog.Warn("message for unknown recipient", "sender", c.Nickname, "recipient", mp.Recipient)
		if err := SendPacketToClient(c, "e", &shared.ErrPacket{Message: "err_unknown_recipient"}); err != nil {
			return fmt.Errorf("failed to send unknown-recipient error: %w", err)
		}
		return nil
	}

	// the sender is bound to the authenticated connection, never trusted
	// from the wire; recipients verify AAD against these authoritative names.
	mp.Sender = c.Nickname

	if err := SendPacketToClient(target, "m", mp); err != nil {
		return fmt.Errorf("failed to deliver message to %q: %w", mp.Recipient, err)
	}
	return nil
}

// HandlePresence handles incoming presence packets. Only verified clients
// may announce presence; the status is validated and the nickname is bound
// to the authenticated connection, never trusted from the wire. The update
// is fanned out to all connected clients via the hub broadcast.
func HandlePresence(h *Hub, c *ServerClient, p shared.GeneralPacket) error {
	var pp shared.PresencePacket
	if err := json.Unmarshal(p.Message, &pp); err != nil {
		return fmt.Errorf("malformed presence: %w", err)
	}
	// only authenticated clients may announce presence
	if !c.Verified || c.Nickname == "" {
		slog.Warn("presence from unverified connection rejected", "client", c.ID)
		if err := SendPacketToClient(c, "e", &shared.ErrPacket{Message: "err_unverified_sender"}); err != nil {
			return fmt.Errorf("failed to send unverified-sender error: %w", err)
		}
		return nil
	}

	if !shared.ValidPresenceStatus(pp.Status) {
		slog.Warn("invalid presence status rejected", "sender", c.Nickname, "status", pp.Status)
		if err := SendPacketToClient(c, "e", &shared.ErrPacket{Message: "err_invalid_presence_status"}); err != nil {
			return fmt.Errorf("failed to send invalid-status error: %w", err)
		}
		return nil
	}

	// the nickname is bound to the authenticated connection, never the wire
	h.Broadcast("p", shared.PresencePacket{Nickname: c.Nickname, Status: pp.Status})
	return nil
}

// HandleJoin rejects client-originated join announcements. The directory is
// server-authoritative: joins are only emitted by completeHandshake after a
// verified handshake, so clients cannot poison peers' HPKE key directories.
func HandleJoin(h *Hub, c *ServerClient, p shared.GeneralPacket) error {
	slog.Warn("rejected client-originated join announcement", "client", c.ID, "nickname", c.Nickname)
	if err := SendPacketToClient(c, "e", &shared.ErrPacket{Message: "err_server_authoritative_directory"}); err != nil {
		return fmt.Errorf("failed to send join-rejection error: %w", err)
	}
	return nil
}

// HandleLeave rejects client-originated leave announcements. Leave packets are
// emitted by the server itself when a verified connection drops.
func HandleLeave(h *Hub, c *ServerClient, p shared.GeneralPacket) error {
	slog.Warn("rejected client-originated leave announcement", "client", c.ID, "nickname", c.Nickname)
	if err := SendPacketToClient(c, "e", &shared.ErrPacket{Message: "err_server_authoritative_directory"}); err != nil {
		return fmt.Errorf("failed to send leave-rejection error: %w", err)
	}
	return nil
}
