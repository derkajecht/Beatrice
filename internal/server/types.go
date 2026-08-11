package server

import (
	"sync"

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
	TuiChan  chan []byte
}

// Hub represents the server-side hub for managing connections and data
type Hub struct {
	mu              sync.RWMutex
	clients         map[string]*ServerClient
	addClientChn    chan *ServerClient
	broadcastChn    chan []byte
	removeClientChn chan *ServerClient
}

func NewHub() *Hub {
	return &Hub{
		clients:         make(map[string]*ServerClient),
		addClientChn:    make(chan *ServerClient),
		removeClientChn: make(chan *ServerClient),
		broadcastChn:    make(chan []byte),
	}
}
