package client

import (
	"crypto/hpke"

	"github.com/coder/websocket"
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

// Stores the pubkey, privkey, nonces seen, and key cache
type CryptoPacket struct {
	Suite      SuiteConfig          `json:"s"`
	PrivKey    []byte               `json:"priv"`
	PubKey     []byte               `json:"pub"`
	SeenNonces *deque.Deque[string] `json:"sn"`
	KeyCache   map[string]string    `json:"kc"`
}

// Stores major parts of the user's information
type User struct {
	Conn           *websocket.Conn   `json:"-"`
	Nickname       string            `json:"n"`
	Crypto         CryptoPacket      `json:"c"`
	ConnectedUsers map[string][]byte `json:"cu"`
	TuiChan        chan []byte
}
