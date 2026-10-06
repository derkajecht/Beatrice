package server

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/derkajecht/Beatrice/src/shared"
)

type InnerNonce struct {
	Nonce string
	Exp   time.Time
}

type Nonces struct {
	mu     sync.Mutex
	nonces map[string]InnerNonce // 	map of nonces with their expiration time
}

func NonceManager() *Nonces {
	// create a new Nonces instance
	nonces := &Nonces{
		nonces: make(map[string]InnerNonce),
	}
	go nonces.cleanupNonces()
	return nonces
}

// Issue generates and stores a new active nonce (valid for 30s)
func (m *Nonces) Issue(c *ServerClient) error {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return err
	}
	nonce := hex.EncodeToString(bytes)

	// store the nonce in the ServerClient struct
	c.NonceKey = nonce

	m.mu.Lock()
	defer m.mu.Unlock()
	m.nonces[c.Nickname] = InnerNonce{
		Nonce: nonce,
		Exp:   time.Now(),
	}

	return nil
}

// Consume checks if the input nonce exists and has been assigned by the server. If yes, check that
// it hasn't expired whilst the check occurs (if it has, then cleanup will catch and delete it).
// If no, delete it from the nonce list
func (m *Nonces) Consume(nickname, nonce string) error {
	if nonce == "" {
		return fmt.Errorf("nonce is empty")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// check if nonce exists in the map
	_, exists := m.nonces[nickname]
	if !exists {
		return fmt.Errorf("nonce does not exist in list. could have been assigned from another machine - WARNING") // Replayed, unknown, or already burned
	}

	// delete it from the map immediately
	delete(m.nonces, nonce)

	return nil
}

func (m *Nonces) cleanupNonces() {
	ticker := time.NewTicker(1 * time.Minute)
	for range ticker.C {
		m.mu.Lock()
		now := time.Now()
		for _, iN := range m.nonces {
			if now.After(iN.Exp) {
				delete(m.nonces, iN.Nonce) // Remove nonces that clients abandoned
			}
		}
		m.mu.Unlock()
	}
}

// CheckChallenge verifies a client's challenge response: the nonce must be one
// we issued and unused (replay protection), and the signature must prove
// ownership of the stored public key.
func CheckChallenge(cr *shared.ChallengeResponse, c *ServerClient, h *Hub) bool {
	// Nonce must be one we issued and must not have been used before.
	if err := h.nonces.Consume(c.Nickname, cr.Nonce); err != nil {
		return false
	}
	// The length check prevents the remote ed25519.Verify panic for malformed
	// identity keys.
	// Signature must prove ownership of the stored public key.
	if len(c.PubKey) != 32 {
		return false
	}
	return ed25519.Verify(c.PubKey, []byte(cr.Nonce), cr.Signature)
}
