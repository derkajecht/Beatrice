package shared

import (
	"encoding/json"
)

// GeneralPacket acts as the global Envelope for all network communication.
// Read this first to determine the inner packet routing.
type GeneralPacket struct {
	Type    string          `json:"t"`
	Message json.RawMessage `json:"m"`
}

type HandshakePacket struct {
	Nickname string `json:"n"`
	PubKey   []byte `json:"k"` // Base64 PEM Identity Key
}

type ChallengePacket struct {
	Nickname    string `json:"n"`
	PendingAuth string `json:"p"` // Challenge payload to be signed by client
	PubKey      []byte `json:"pk"`
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
	PubKey   []byte `json:"k"`
}

type DirPacket struct {
	CurrentUsers map[string][]byte `json:"cu"` // map[Nickname]PublicKey
}

type LeavePacket struct {
	Nickname string `json:"n"`
	PubKey   []byte `json:"k"`
}

type ErrPacket struct {
	Message string `json:"m"`
}

type PubKey struct {
	PubKey []byte `json:"k"`
}
