// Package crypto provides cryptographic utilities for Beatrice.
// It includes functions for generating new key pairs, serializing keys, and more.
// Called by the client on startup to generate a new key pair and store it memory.
package crypto

import (
	"fmt"

	"crypto/hpke"

	"github.com/derkajecht/Beatrice/internal/shared/types"
	"github.com/gammazero/deque"
)

//

func NewCryptoSuite() *types.SuiteConfig {
	return &types.SuiteConfig{
		KEM:  hpke.MLKEM768X25519(),
		KDF:  hpke.HKDFSHA512(),
		AEAD: hpke.AES256GCM(),
		Info: []byte("Beatrice"),
	}
}

// NewUserSession returns a new ephemeral key pair and session.
func NewUserSession() (*types.PubKey, *types.PrivKey, *types.CryptoPacket, error) {
	suite := NewCryptoSuite()

	privKey, err := suite.KEM.GenerateKey()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	pubBytes := privKey.PublicKey().Bytes()
	privBytes, err := privKey.Bytes()
	if err != nil {
		// NOTE: should i be returning this or just logging it?
		return nil, nil, nil, fmt.Errorf("failed to serialize private key bytes: %w", err)
	}

	privKeyPacket := types.PrivKey{
		PrivKey: privBytes,
	}
	pubKeyPacket := types.PubKey{
		PubKey: pubBytes,
	}
	cryptoPacket := &types.CryptoPacket{
		Suite:      suite,
		PrivKey:    privKeyPacket,
		PubKey:     pubKeyPacket,
		SeenNonces: new(deque.Deque[string]),
		KeyCache:   make(map[string]string),
	}

	return &cryptoPacket.PubKey, &cryptoPacket.PrivKey, cryptoPacket, nil
}
