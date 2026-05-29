package models

import (
	"net"
	"sync"

	"github.com/derkajecht/Beatrice/internal/models"
)

// i know some of these structs are reused under a different name
// but for readability i'll leave them as they are for now...

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
	Type          string `json:"t"`
	Current_Users string `json:"p"`
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
	Type    string `json:"t"`
	Message string `json:"m"`
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

// NewErrPacket creates a new error packet struct with the given error message
func NewErrPacket(errMsg string) *models.ErrPacket {
	return &models.ErrPacket{
		Type:    "e",
		Message: errMsg,
	}
}

// NewDirPacket creates a new dir packet struct with the given user list
func NewDirPacket(GetUserList []string) *models.DirPacket {
	return &models.DirPacket{
		Type:          "d",
		Current_Users: GetUserList,
	}
}

// GetUserList returns a list of nicknames and their public keys for all connected users
// except the given nickname
func (r *Room) GetUserList(nickname string) []string {
	r.Lock()
	defer r.Unlock()

	var userList []string
	for _, client := range r.Clients {
		if client.Nickname != nickname {
			userList = append(userList, client.Nickname, client.PubKey)
		}
	}
	return userList
}
