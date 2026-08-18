package client

import (
	"crypto/ed25519"
	"crypto/hpke"
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
}

func NewUser() *User {
	return &User{
		ConnectedUsers: make(map[string][]byte),
		TuiChan:        make(chan shared.GeneralPacket, 100),
	}
}
