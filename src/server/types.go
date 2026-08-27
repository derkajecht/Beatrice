package server

import (
	"sync"
	"time"

	"github.com/coder/websocket"
)

// -------------------------------------------------------------------
// Server Memory Tracking Models
// -------------------------------------------------------------------

// ServerClient represents a fully authenticated server-side active connection tracking state
type ServerClient struct {
	ID         string
	Conn       *websocket.Conn
	Nickname   string
	PubKey     []byte // ed25519 identity public key (DB/TOFU + challenge verification)
	HPKEPubKey []byte // HPKE KEM public key, distributed to peers via Dir/Join packets
	Verified   bool
}

func NewServerClient(ID string, conn *websocket.Conn) *ServerClient {
	return &ServerClient{
		ID:       ID,
		Conn:     conn,
		PubKey:   []byte{},
		Verified: false, // no trust by default. proven by challenge packet success
	}
}

// broadcastMsg packages a client and raw message data for the hub broadcaster.
type broadcastMsg struct {
	conn *ServerClient
	data []byte
}

// Hub represents the server-side hub for managing connections and data
type Hub struct {
	mu                sync.RWMutex
	clients           map[string]*ServerClient
	broadcastChn      chan broadcastMsg
	db                *DatabaseInfo
	nonces            *Nonces
	InactivityTimeout time.Duration
}

func NewHub() *Hub {
	return &Hub{
		clients:           make(map[string]*ServerClient),
		broadcastChn:      make(chan broadcastMsg, 64),
		nonces:            NonceManager(),
		InactivityTimeout: 180 * time.Second, // 3 minutes timeout
	}
}
