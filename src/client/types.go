package client

import (
	"crypto/ed25519"
	"sync"

	"github.com/cloudflare/circl/hpke"
	"github.com/cloudflare/circl/kem"

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
	HPKESuite       hpke.Suite
	Suite           SuiteConfig        `json:"s"`
	PrivKey         kem.PrivateKey     `json:"priv"`
	PubKey          kem.PublicKey      `json:"pub"`
	IdentityPubKey  []byte             `json:"idpub"`
	IdentityPrivKey ed25519.PrivateKey `json:"-"`
	// TODO: SeenNonces is never initialized or consulted, so encrypted message
	// replays are accepted indefinitely.
	SeenNonces *deque.Deque[string] `json:"sn"`
	KeyCache   map[string]string    `json:"kc"`
}

// Stores major parts of the user's information
type User struct {
	mu             sync.RWMutex              `json:"-"`
	Conn           *websocket.Conn           `json:"-"`
	Nickname       string                    `json:"n"`
	Crypto         CryptoPacket              `json:"c"`
	ConnectedUsers map[string]kem.PublicKey  `json:"cu"`
	TuiChan        chan shared.GeneralPacket `json:"-"`
	Addr           string

	// Lazily reconstructed own HPKE private key for inbound decryption,
	// rebuilt once from CryptoPacket.PrivKey.
	privKey     kem.PrivateKey `json:"-"`
	privKeyOnce sync.Once      `json:"-"`
	privKeyErr  error          `json:"-"`
}

func NewUser() *User {
	return &User{
		ConnectedUsers: make(map[string]kem.PublicKey),
		TuiChan:        make(chan shared.GeneralPacket, 100),
	}
}

// OwnPrivateKey reconstructs and caches the user's own HPKE private key from
// CryptoPacket.PrivKey so inbound messages can be opened. The key is parsed
// once; subsequent calls reuse the cached value.
// func (u *User) OwnPrivateKey() (kem.PrivateKey, error) {
// 	u.privKeyOnce.Do(func() {
// 		u.privKey, u.privKeyErr = u.Crypto.HPKESuite.KEM.NewPrivateKey(u.Crypto.PrivKey)
// 		if u.privKeyErr != nil {
// 			u.privKeyErr = fmt.Errorf("failed to parse own private key: %w", u.privKeyErr)
// 		}
// 	})
// 	return u.privKey, u.privKeyErr
// }

// ApplyDirPacket replaces the connected-users directory with a
// server-authoritative snapshot (map[Nickname]HPKEPublicKey).
func (u *User) ApplyDirPacket(users map[string][]byte, wire shared.WireHPKEPubKey) {
	if users == nil {
		users = make(map[string][]byte)
	}
	usersPacket := make(map[string]kem.PublicKey)

	for nickname, rawKey := range users {
		pubKey, err := DecodePubKey(shared.WireHPKEPubKey{
			KEM: wire.KEM,
			Key: rawKey,
		})
		if err != nil {
			return
		}
		usersPacket[nickname] = pubKey
	}
	u.mu.Lock()
	u.ConnectedUsers = usersPacket
	u.mu.Unlock()
}

// ApplyJoin adds or refreshes a peer's HPKE public key from an authoritative
// join packet.
func (u *User) ApplyJoin(nickname string, pubKey kem.PublicKey) {
	u.mu.Lock()
	if u.ConnectedUsers == nil {
		u.ConnectedUsers = make(map[string]kem.PublicKey)
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
// func (u *User) GetConnectedUserInfo() map[string]kem.PublicKey {
// 	u.mu.RLock()
// 	defer u.mu.RUnlock()
//
// 	result := make(map[string]kem.PublicKey, len(u.ConnectedUsers))
// 	for k, v := range u.ConnectedUsers {
// 		bytesCopy := make(kem.PublicKey, len(v))
// 		copy(bytesCopy, v)
// 		result[k] = v
// 	}
// 	return result
// }
