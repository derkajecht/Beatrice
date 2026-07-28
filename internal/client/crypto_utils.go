// Package crypto provides cryptographic utilities for Beatrice.
// It includes functions for generating new key pairs, serializing keys, and more.
// Called by the client on startup to generate a new key pair and store it memory.
package client

import (
	"fmt"

	"crypto/hpke"

	"github.com/derkajecht/Beatrice/internal/shared"
	"github.com/gammazero/deque"
)

//

func NewCryptoSuite() SuiteConfig {
	return SuiteConfig{
		KEM:  hpke.MLKEM768X25519(),
		KDF:  hpke.HKDFSHA512(),
		AEAD: hpke.AES256GCM(),
		Info: []byte("Beatrice"),
	}
}

func CryptoPacketToPubKey(crypto CryptoPacket) shared.PubKey {
	return shared.PubKey{
		PubKey: crypto.PubKey,
	}
}

// NewUserSession returns a new ephemeral key pair and session.
func NewUserSession() (shared.PubKey, CryptoPacket, error) {
	suite := NewCryptoSuite()

	privKey, err := suite.KEM.GenerateKey()
	if err != nil {
		return shared.PubKey{}, CryptoPacket{}, fmt.Errorf("failed to generate private key: %w", err)
	}

	pubBytes := privKey.PublicKey().Bytes()
	privBytes, err := privKey.Bytes()
	if err != nil {
		// NOTE: should i be returning this or just logging it?
		return shared.PubKey{}, CryptoPacket{}, fmt.Errorf("failed to serialize private key: %w", err)
	}

	// create a new crypto packet with the generated key pair
	// add the public key to the crypto packet and to the shared PubKey struct
	cryptoPacket := CryptoPacket{
		Suite:      suite,
		PrivKey:    privBytes,
		PubKey:     pubBytes,
		SeenNonces: new(deque.Deque[string]),
		KeyCache:   make(map[string]string),
	}

	return shared.PubKey{PubKey: cryptoPacket.PubKey}, cryptoPacket, nil
}
