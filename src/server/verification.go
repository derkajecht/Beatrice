package server

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"github.com/derkajecht/Beatrice/src/shared"
)

type Nonces struct {
	mu     sync.Mutex
	nonces map[string]time.Time // 	map of nonces with their expiration time
}

func NonceManager() *Nonces {
	// create a new Nonces instance
	nonces := &Nonces{
		nonces: make(map[string]time.Time),
	}
	go nonces.cleanupNonces()
	return nonces
}

// Issue generates and stores a new active nonce (valid for 30s)
func (m *Nonces) Issue() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	nonce := hex.EncodeToString(bytes)

	m.mu.Lock()
	defer m.mu.Unlock()
	m.nonces[nonce] = time.Now().Add(30 * time.Second)

	return nonce, nil
}

// Consume checks if a nonce is valid.
// TODO: Burning happens before signature verification and nonces are not bound
// to a connection, so an observer can consume or transplant a challenge.
func (m *Nonces) Consume(nonce string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	exp, exists := m.nonces[nonce]
	if !exists {
		return false // Replayed, unknown, or already burned
	}

	// Verify it didn't expire while waiting
	return time.Now().Before(exp)
}

func (m *Nonces) BurnNonce(nonce string) error {
	if nonce != "" {
		delete(m.nonces, nonce)
	}
	return nil
}

func (m *Nonces) cleanupNonces() {
	ticker := time.NewTicker(1 * time.Minute)
	for range ticker.C {
		m.mu.Lock()
		now := time.Now()
		for nonce, exp := range m.nonces {
			if now.After(exp) {
				delete(m.nonces, nonce) // Remove nonces that clients abandoned
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
	if !h.nonces.Consume(cr.Nonce) {
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
