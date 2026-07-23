package crypto

import (
	"crypto/hpke"
	"crypto/rand"
)

func DeriveKEM() ([]byte, error) {
	// init MLKEM76825519 suite
	kem := hpke.MLKEM768X25519()

	// Generate a random 32 byte key
	ikm := make([]byte, 32)
	_, err := rand.Read(ikm)
	if err != nil {
		return nil, err
	}

	// Derive the private key deterministically
	privKey, err := kem.DeriveKeyPair(ikm)
	if err != nil {
		return nil, err
	}

	// extract the public key
	pubKey := privKey.PublicKey()
	pubBytes := pubKey.Bytes()

	return pubBytes, nil
}

func DeriveKDF() ([]byte, error) {
	// init HKDF suite
	kdf := hpke.HKDFSHA256()

	// Generate a random 32 byte key
	ikm := make([]byte, 32)
	_, err := rand.Read(ikm)
	if err != nil {
		return nil, err
	}


