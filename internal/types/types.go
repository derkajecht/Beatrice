package types

import (
	"encoding/json"
	"net"
	"sync"
)

// GeneralPacket acts as the global Envelope for all network communication.
// Read this first to determine the inner packet routing.
type GeneralPacket struct {
	Type    string          `json:"t"`
	Message json.RawMessage `json:"m"`
}

type HandshakePacket struct {
	Nickname string `json:"n"`
	PubKey   string `json:"k"` // Base64 PEM Identity Key
}

type ChallengePacket struct {
	Nickname    string `json:"n"`
	PendingAuth string `json:"p"` // Challenge payload to be signed by client
	PubKey      string `json:"pk"`
	IsNew       bool   `json:"new"`
}

type MessagePacket struct {
	Recipient string `json:"r"`
	Sender    string `json:"s"`
	IV        string `json:"iv"`  // Base64 Initialization Vector for AES-GCM/CBC
	AESKey    string `json:"k"`   // Ephemeral AES key encrypted via Recipient's PubKey (Base64)
	Content   string `json:"c"`   // Encrypted message payload payload (Base64)
	Signature string `json:"sig"` // Sender's signature verifying authenticity
}

type JoinPacket struct {
	Nickname string `json:"n"`
	PubKey   string `json:"k"`
}

type DirPacket struct {
	CurrentUsers map[string]string `json:"cu"` // map[Nickname]PublicKey
}

type LeavePacket struct {
	Nickname string `json:"n"`
	PubKey   string `json:"k"`
}

type ErrPacket struct {
	Message string `json:"m"`
}

type SuccessPacket struct {
	Message string `json:"m"`
}

// -------------------------------------------------------------------
// Server Memory Tracking Models
// -------------------------------------------------------------------

// Client represents a fully authenticated server-side active connection tracking state
type Client struct {
	Conn     net.Conn
	Nickname string
	PubKey   string
}

// Room represents a thread-safe chat room instance
type Room struct {
	sync.RWMutex // Upgraded to RWMutex for high-performance concurrent reads
	Clients      map[net.Conn]*Client
}

// NewRoom acts as a reliable constructor for room instances
func NewRoom() *Room {
	return &Room{
		Clients: make(map[net.Conn]*Client),
	}
}

// ChatRoom is the global thread-safe active memory map
var ChatRoom = NewRoom()
