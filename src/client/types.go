package client

import (
	"crypto/ed25519"
	"crypto/hpke"
	"fmt"
	"sync"

	"github.com/coder/websocket"
	"github.com/derkajecht/Beatrice/src/shared"
	"github.com/gammazero/deque"
)

// -------------------------------------------------------------------
// User Models
// -------------------------------------------------------------------

type SuiteConfig struct {
	KEM  hpke.KEM  `json:"kem"`
	KDF  hpke.KDF  `json:"kdf"`
	AEAD hpke.AEAD `json:"aead"`
	Info []byte    `json:"info"`
}

// Stores the pubkey, privkey, identity key, nonces seen, and key cache
type CryptoPacket struct {
	Suite           SuiteConfig          `json:"s"`
	PrivKey         []byte               `json:"priv"`
	PubKey          []byte               `json:"pub"`
	IdentityPubKey  []byte               `json:"idpub"`
	IdentityPrivKey ed25519.PrivateKey   `json:"-"`
	SeenNonces      *deque.Deque[string] `json:"sn"`
	KeyCache        map[string]string    `json:"kc"`
}

// Stores major parts of the user's information
type User struct {
	mu             sync.RWMutex              `json:"-"`
	Conn           *websocket.Conn           `json:"-"`
	Nickname       string                    `json:"n"`
	Crypto         CryptoPacket              `json:"c"`
	ConnectedUsers map[string][]byte         `json:"cu"`
	TuiChan        chan shared.GeneralPacket `json:"-"`
	Addr           string

	// Lazily reconstructed own HPKE private key for inbound decryption,
	// rebuilt once from CryptoPacket.PrivKey.
	privKey     hpke.PrivateKey `json:"-"`
	privKeyOnce sync.Once       `json:"-"`
	privKeyErr  error           `json:"-"`
}

func NewUser() *User {
	return &User{
		ConnectedUsers: make(map[string][]byte),
		TuiChan:        make(chan shared.GeneralPacket, 100),
	}
}

// OwnPrivateKey reconstructs and caches the user's own HPKE private key from
// CryptoPacket.PrivKey so inbound messages can be opened. The key is parsed
// once; subsequent calls reuse the cached value.
func (u *User) OwnPrivateKey() (hpke.PrivateKey, error) {
	u.privKeyOnce.Do(func() {
		u.privKey, u.privKeyErr = u.Crypto.Suite.KEM.NewPrivateKey(u.Crypto.PrivKey)
		if u.privKeyErr != nil {
			u.privKeyErr = fmt.Errorf("failed to parse own private key: %w", u.privKeyErr)
		}
	})
	return u.privKey, u.privKeyErr
}

// ApplyDirPacket replaces the connected-users directory with a
// server-authoritative snapshot (map[Nickname]HPKEPublicKey).
func (u *User) ApplyDirPacket(users map[string][]byte) {
	if users == nil {
		users = make(map[string][]byte)
	}
	u.mu.Lock()
	u.ConnectedUsers = users
	u.mu.Unlock()
}

// ApplyJoin adds or refreshes a peer's HPKE public key from an authoritative
// join packet.
func (u *User) ApplyJoin(nickname string, pubKey []byte) {
	u.mu.Lock()
	if u.ConnectedUsers == nil {
		u.ConnectedUsers = make(map[string][]byte)
	}
	u.ConnectedUsers[nickname] = pubKey
	u.mu.Unlock()
}

// ApplyLeave removes a departed peer from the connected-users directory.
func (u *User) ApplyLeave(nickname string) {
	u.mu.Lock()
	delete(u.ConnectedUsers, nickname)
	u.mu.Unlock()
}

// GetConnectedUserInfo generates a list showing the currently connected user list
func (u *User) GetConnectedUserInfo() map[string][]byte {
	u.mu.RLock()
	defer u.mu.RUnlock()

	result := make(map[string][]byte, len(u.ConnectedUsers))
	for k, v := range u.ConnectedUsers {
		bytesCopy := make([]byte, len(v))
		copy(bytesCopy, v)
		result[k] = bytesCopy
	}
	return result
}
