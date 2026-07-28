package server

import (
	"github.com/coder/websocket"
)

// -------------------------------------------------------------------
// Server Memory Tracking Models
// -------------------------------------------------------------------

type Server struct {
	ActiveConnections  map[string]map[string]string `json:"ac"`  // map of current active users - "jordan": {"conn": 1234, "pk": "-----BEGIN PUBLIC KEY..."}
	PendingConnections map[string]string            `json:"pc"`  // once handshake is confirmed, this will store "jordan": "-----BEGIN PUBLIC KEY..." and then be passed to connection method
	DatabasePath       string                       `json:"dbp"` // path to the database, stored in memory
}

// Client represents a fully authenticated server-side active connection tracking state
type ServerClient struct {
	Conn     *websocket.Conn
	Nickname string
	PubKey   []byte
	TuiChan  chan []byte
}

type Hub struct {
	clients      map[*websocket.Conn]*websocket.Client
	register     chan *websocket.Conn
	broadcast    chan []byte
	dm           chan []byte
	handshake    chan *websocket.Conn
	deleteClient chan *websocket.Conn
}

func NewHub() *Hub {
	return &Hub{
		clients:      make(map[*websocket.Conn]*websocket.Client),
		broadcast:    make(chan []byte),
		dm:           make(chan []byte),
		handshake:    make(chan *websocket.Conn),
		deleteClient: make(chan *websocket.Conn),
	}
}
