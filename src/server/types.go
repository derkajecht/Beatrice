package server

import (
	"sync"
	"time"

	"github.com/coder/websocket"
)

// -------------------------------------------------------------------
// Server Memory Tracking Models
// -------------------------------------------------------------------

// Client represents a fully authenticated server-side active connection tracking state
type ServerClient struct {
	ID       string
	Conn     *websocket.Conn
	Nickname string
	PubKey   []byte
}

func NewServerClient(ID string, conn *websocket.Conn) *ServerClient {
	return &ServerClient{
		ID:     ID,
		Conn:   conn,
		PubKey: []byte{},
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
	InactivityTimeout time.Duration
}

func NewHub() *Hub {
	return &Hub{
		clients:      make(map[string]*ServerClient),
		broadcastChn: make(chan broadcastMsg, 64),
		// TODO: make this configurable
		InactivityTimeout: 180 * time.Second, // 3 minutes timeout
	}
}
