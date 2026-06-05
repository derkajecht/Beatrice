package types

import (
	"net"
	"sync"
)

type Envelope struct {
	Type string `json:"t"`
}

type HandshakePacket struct {
	Type     string `json:"t"`
	Nickname string `json:"n"`
	PubKey   string `json:"k"`
}

type MessagePacket struct {
	Type      string `json:"t"`
	Recipient string `json:"r"`
	Sender    string `json:"s"`
	IV        string `json:"iv"`
	AESKey    []byte `json:"k"` // AES key encrypted with recipient's public key
	Content   []byte `json:"c"` // Encrypted message blob
	Signature string `json:"sig"`
}

type JoinPacket struct {
	Type     string `json:"t"`
	Nickname string `json:"n"`
	PubKey   string `json:"k"`
}

type DirPacket struct {
	Type          string            `json:"t"`
	Current_Users map[string]string `json:"cu"`
}

type ChallengePacket struct {
	Type         string `json:"t"`
	Nickname     string `json:"n"`
	Pending_Auth string `json:"p"`
	PubKey       string `json:"pk"`
	IsNew        bool   `json:"new"`
}

type LeavePacket struct {
	Type     string `json:"t"`
	Nickname string `json:"n"`
	PubKey   string `json:"k"`
}

type ErrPacket struct {
	Type    string `json:"t"`
	Message string `json:"m"`
}

type SuccessPacket struct {
	Type    string            `json:"t"`
	Message map[string]string `json:"m"`
}

type GeneralPacket struct {
	Type    string            `json:"t"`
	Message map[string]string `json:"m"`
}

// Client represents a client connection
type Client struct {
	Conn     net.Conn
	Nickname string
	PubKey   string
}

// Room represents a chat room
type Room struct {
	sync.Mutex
	Clients map[net.Conn]*Client
}

// ChatRoom is the global chat room
var ChatRoom = &Room{
	Clients: make(map[net.Conn]*Client),
}
