// Package crypto provides cryptographic utilities for Beatrice.
// It includes functions for generating new key pairs, serializing keys, and more.
// Called by the client on startup to generate a new key pair and store it memory.
package client

import (
	"crypto/ed25519"
	"crypto/hpke"
	"crypto/rand"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/derkajecht/Beatrice/src/shared"
)

type EncryptedTarget struct {
	Nickname string
	Enc      []byte
	CT       []byte
}

// NewCryptoSuite returns a new crypto suite with the default settings.
func NewCryptoSuite() SuiteConfig {
	return SuiteConfig{
		KEM:  hpke.MLKEM768X25519(),
		KDF:  hpke.HKDFSHA512(),
		AEAD: hpke.AES256GCM(),
		Info: []byte("Beatrice"),
	}
}

func NewCryptoPacket(suite SuiteConfig, priv, pub, idPub []byte, idPriv ed25519.PrivateKey) *CryptoPacket {
	return &CryptoPacket{
		Suite:           suite,
		PrivKey:         priv,
		PubKey:          pub,
		IdentityPubKey:  idPub,
		IdentityPrivKey: idPriv,
		// SeenNonces:      new(deque.Deque[string]),
		KeyCache: make(map[string]string),
	}
}

// identityKeyPath returns the filesystem path for the persistent ed25519
// identity key, mirroring the server's data directory per OS.
func identityKeyPath() (string, error) {
	var dir string
	j := shared.CheckOS()
	switch j {
	case shared.Darwin:
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, "Library", "Application Support", "Beatrice")
	case shared.Windows:
		dir = os.Getenv("APPDATA")
		if dir == "" {
			return "", fmt.Errorf("APPDATA not set")
		}
		dir = filepath.Join(dir, "Beatrice")
	default: // linux and other os
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, ".beatrice")
	}
	return filepath.Join(dir, "identity.key"), nil
}

// loadOrCreateIdentityKey loads the persistent ed25519 identity key from disk,
// or generates and saves a new one on first run. Returning users keep the same
// key across sessions so the server can verify their identity on rejoin.
func loadOrCreateIdentityKey() (ed25519.PublicKey, ed25519.PrivateKey, error) {
	path, err := identityKeyPath()
	if err != nil {
		return nil, nil, err
	}

	// load existing key pair from disk
	if data, err := os.ReadFile(path); err == nil && len(data) == ed25519.PrivateKeySize {
		priv := ed25519.PrivateKey(data)
		return priv.Public().(ed25519.PublicKey), priv, nil
	}

	// derive new key pair
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate identity key: %w", err)
	}
	// create dir based on path and write the private key to disk
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, nil, fmt.Errorf("failed to create identity key dir: %w", err)
	}
	if err := os.WriteFile(path, priv, 0o600); err != nil {
		return nil, nil, fmt.Errorf("failed to save identity key: %w", err)
	}
	return pub, priv, nil
}

// NewUserSession returns a new ephemeral key pair, an ed25519 identity key pair,
// and a session. Returns an error on failure to generate either key pair.
func NewUserSession(ephemeral bool) (*CryptoPacket, error) {
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

	// ed25519 identity key used to sign challenge responses (and later messages).
	// Persisted to disk so returning users present the same key on rejoin.
	var idPub ed25519.PublicKey
	var idPriv ed25519.PrivateKey
	if ephemeral {
		idPub, idPriv, err = ed25519.GenerateKey(rand.Reader)
	} else {
		idPub, idPriv, err = loadOrCreateIdentityKey()
		if err != nil {
			return nil, err
		}
	}

	// create a new crypto packet with the generated key pairs
	return NewCryptoPacket(suite, privBytes, pubBytes, idPub, idPriv), nil
}

// EncryptMessage encrypts content for each target nickname using that user's
// stored HPKE public key from the local connected-users directory. It returns
// one EncryptedTarget per successfully encrypted recipient. Targets that are
// unknown or fail to encrypt are logged and reported in the returned error;
// the successful subset is still returned so callers can deliver to reachable
// recipients. The sender's own nickname is always skipped.
func EncryptMessage(u *User, targets []string, content string) ([]EncryptedTarget, error) {
	suite := u.Crypto.Suite
	users := u.GetConnectedUserInfo()

	out := make([]EncryptedTarget, 0, len(targets))
	var failed []string

	for _, target := range targets {
		if target == u.Nickname {
			continue // never encrypt to self
		}
		pubBytes, ok := users[target]
		if !ok || len(pubBytes) == 0 {
			slog.Error("no public key for target", "user", target)
			failed = append(failed, target)
			continue
		}

		pub, err := suite.KEM.NewPublicKey(pubBytes)
		if err != nil {
			slog.Error("failed to parse public key", "user", target, "err", err)
			failed = append(failed, target)
			continue
		}

		enc, sender, err := hpke.NewSender(pub, suite.KDF, suite.AEAD, suite.Info)
		if err != nil {
			slog.Error("failed to create HPKE sender", "user", target, "err", err)
			failed = append(failed, target)
			continue
		}

		// AAD binds the ciphertext to the authenticated sender/recipient pair;
		// metadata tampering in transit causes Open to fail on the receiver.
		ct, err := sender.Seal(shared.MessageAAD(u.Nickname, target), []byte(content))
		if err != nil {
			slog.Error("failed to seal message", "user", target, "err", err)
			failed = append(failed, target)
			continue
		}

		out = append(out, EncryptedTarget{Nickname: target, Enc: enc, CT: ct})
	}

	if len(failed) > 0 {
		return out, fmt.Errorf("failed to encrypt for: %s", strings.Join(failed, ", "))
	}
	return out, nil
}

// DecryptMessage opens an inbound HPKE message addressed to this user using
// the user's own reconstructed private key. sender/recipient must be the
// authoritative names from the routed packet; they are bound as AAD and must
// match what the sender sealed. The server never sees plaintext.
func DecryptMessage(u *User, enc, ct []byte, sender, recipient string) (string, error) {
	if len(enc) == 0 || len(ct) == 0 {
		return "", fmt.Errorf("message missing enc or ct. both needed for successful decryption")
	}

	priv, err := u.OwnPrivateKey()
	if err != nil {
		return "", fmt.Errorf("failed to reconstruct own private key: %w", err)
	}

	suite := u.Crypto.Suite
	r, err := hpke.NewRecipient(enc, priv, suite.KDF, suite.AEAD, suite.Info)
	if err != nil {
		return "", fmt.Errorf("failed to create HPKE recipient: %w", err)
	}

	pt, err := r.Open(shared.MessageAAD(sender, recipient), ct)
	if err != nil {
		return "", fmt.Errorf("failed to open ciphertext: %w", err)
	}
	return string(pt), nil
}
