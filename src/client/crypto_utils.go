// Package crypto provides cryptographic utilities for Beatrice.
// It includes functions for generating new key pairs, serializing keys, and more.
// Called by the client on startup to generate a new key pair and store it memory.
package client

import (
	"fmt"

	"crypto/hpke"

	"github.com/gammazero/deque"
)

// NewCryptoSuite returns a new crypto suite with the default settings.
func NewCryptoSuite() SuiteConfig {
	return SuiteConfig{
		KEM:  hpke.MLKEM768X25519(),
		KDF:  hpke.HKDFSHA512(),
		AEAD: hpke.AES256GCM(),
		Info: []byte("Beatrice"),
	}
}

func NewCryptoPacket(suite SuiteConfig, priv, pub []byte) *CryptoPacket {
	return &CryptoPacket{
		Suite:      suite,
		PrivKey:    priv,
		PubKey:     pub,
		SeenNonces: new(deque.Deque[string]),
		KeyCache:   make(map[string]string),
	}
}

// NewUserSession returns a new ephemeral key pair and session.
// this would return on failure to generate a new key pair
func NewUserSession() (*CryptoPacket, error) {
	suite := NewCryptoSuite()

	// generate private key using the crypto suite
	privKey, err := suite.KEM.GenerateKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %w", err)
	}
	if privKey == nil {
		return nil, fmt.Errorf("private key is nil")
	}

	// derive the public key from the private key and convert both to bytes
	pubBytes := privKey.PublicKey().Bytes()
	privBytes, err := privKey.Bytes()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize private key: %w", err)
	}

	// create a new crypto packet with the generated key pair
	// add the public key to the crypto packet and to the shared PubKey struct
	return NewCryptoPacket(suite, privBytes, pubBytes), nil
}
