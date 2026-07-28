package types

import (
	"crypto/hpke"
	"encoding/json"
	"sync"

	"github.com/coder/websocket"
	"github.com/gammazero/deque"
)

// GeneralPacket acts as the global Envelope for all network communication.
// Read this first to determine the inner packet routing.
type GeneralPacket struct {
	Type    string          `json:"t"`
	Message json.RawMessage `json:"m"`
}

type HandshakePacket struct {
	Nickname string `json:"n"`
	PubKey   PubKey `json:"k"` // Base64 PEM Identity Key
}

type ChallengePacket struct {
	Nickname    string `json:"n"`
	PendingAuth string `json:"p"` // Challenge payload to be signed by client
	PubKey      PubKey `json:"pk"`
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
	PubKey   PubKey `json:"k"`
}

type DirPacket struct {
	CurrentUsers map[string][]byte `json:"cu"` // map[Nickname]PublicKey
}

type LeavePacket struct {
	Nickname string `json:"n"`
	PubKey   PubKey `json:"k"`
}

type ErrPacket struct {
	Message string `json:"m"`
}

type SuccessPacket struct {
	Message string `json:"m"`
}

// -------------------------------------------------------------------
// User Models
// -------------------------------------------------------------------
// Stores the pub key and nickname of connected users
type ConnectedUsers struct {
	Users map[string]string `json:"cu"` // map[Nickname]PublicKey
}

// PubKey holds public key bytes for JSON serialization.
type PubKey struct {
	PubKey []byte `json:"k"`
}

// PrivKey holds private key bytes for JSON serialization.
type PrivKey struct {
	PrivKey []byte `json:"k"`
}

type SuiteConfig struct {
	KEM  hpke.KEM  `json:"kem"`
	KDF  hpke.KDF  `json:"kdf"`
	AEAD hpke.AEAD `json:"aead"`
	Info []byte    `json:"info"`
}

// Stores the pubkey, privkey, nonces seen, and key cache
type CryptoPacket struct {
	Suite      *SuiteConfig         `json:"s"`
	PrivKey    PrivKey              `json:"priv"`
	PubKey     PubKey               `json:"pub"`
	SeenNonces *deque.Deque[string] `json:"sn"`
	KeyCache   map[string]string    `json:"kc"`
}

// Stores major parts of the user's information
type User struct {
	Nickname       string         `json:"n"`
	Crypto         CryptoPacket   `json:"c"`
	ConnectedUsers ConnectedUsers `json:"cu"`
}

// -------------------------------------------------------------------
// Server Memory Tracking Models
// -------------------------------------------------------------------

type Server struct {
	ActiveConnections  map[string]map[string]string `json:"ac"`  // map of current active users - "jordan": {"conn": 1234, "pk": "-----BEGIN PUBLIC KEY..."}
	PendingConnections map[string]string            `json:"pc"`  // once handshake is confirmed, this will store "jordan": "-----BEGIN PUBLIC KEY..." and then be passed to connection method
	DatabasePath       string                       `json:"dbp"` // path to the database, stored in memory
}

// Room represents a thread-safe chat room instance
type Room struct {
	sync.RWMutex // Upgraded to RWMutex for high-performance concurrent reads
	Clients      map[*websocket.Conn]*Client
}

// NewRoom acts as a reliable constructor for room instances
func NewRoom() *Room {
	return &Room{
		Clients: make(map[*websocket.Conn]*Client),
	}
}

// Client represents a fully authenticated server-side active connection tracking state
type Client struct {
	Conn     *websocket.Conn
	Nickname string
	PubKey   PubKey
	TuiChan  chan []byte
}

// AddClient adds a new client to the room
func (r *Room) AddClient(conn *websocket.Conn, nickname string, pubKey PubKey) *Client {
	client := &Client{
		Conn:     conn,
		Nickname: nickname,
		PubKey:   pubKey,
	}
	r.Lock()
	defer r.Unlock()
	r.Clients[conn] = client

	return client
}

// GetClient returns a client from the room
func (r *Room) GetClient(conn *websocket.Conn) (*Client, bool) {
	r.RLock()
	defer r.RUnlock()
	client, ok := r.Clients[conn]
	return client, ok
}

// RemoveClient removes a client from the room
func (r *Room) RemoveClient(conn *websocket.Conn) {
	r.Lock()
	defer r.Unlock()
	delete(r.Clients, conn)
}

// ChatRoom is the global thread-safe active memory map
var ChatRoom = NewRoom()
