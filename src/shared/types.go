package shared

import (
	"encoding/json"
	"time"
)

// GeneralPacket acts as the global Envelope for all network communication.
// Read this first to determine the inner packet routing.
type GeneralPacket struct {
	Type    string          `json:"t"`
	Message json.RawMessage `json:"m"`
}

type NicknameUpdatePacket struct {
	Nickname string `json:"n"`
}

type HandshakePacket struct {
	Nickname   string `json:"n"`
	PubKey     []byte `json:"k"`  // Base64 PEM Identity (ed25519) Key — used for DB/TOFU + challenge verification
	HPKEPubKey []byte `json:"hk"` // HPKE KEM public key — distributed to peers for message encryption
}

type ChallengePacket struct {
	Nickname    string `json:"n"`
	PendingAuth string `json:"p"` // Challenge payload to be signed by client
	PubKey      []byte `json:"pk"`
	IsNew       bool   `json:"new"`
}

// ChallengeResponse is the client's reply to a ChallengePacket: the nonce
// signed with the client's identity (ed25519) private key.
type ChallengeResponse struct {
	Signature []byte `json:"sig"`
	Nonce     string `json:"n"`
}

// MessagePacket is the encrypted wire representation of a chat message.
// The payload is HPKE-sealed per recipient (Enc/CT); the server routes this
// packet without ever being able to decrypt it. Plaintext never travels here.
type MessagePacket struct {
	Recipient string `json:"r"`
	Sender    string `json:"s"`
	Enc       []byte `json:"enc"` // HPKE encapsulation key (Base64 on wire)
	CT        []byte `json:"ct"`  // HPKE ciphertext (Base64 on wire)
	Signature string `json:"sig"` // Sender's signature verifying authenticity
	Time      time.Time
}

// MessageAAD derives the deterministic additional authenticated data bound
// into every HPKE seal/open call. It ties each ciphertext to its sender and
// recipient, so tampering with routing metadata causes decryption failure.
// Must be used identically on both sides.
func MessageAAD(sender, recipient string) []byte {
	return []byte("beatrice:m\x00" + sender + "\x00" + recipient)
}

// DecryptedMessage is the local, post-decryption representation handed from
// the client backend to the TUI. It is never placed on the wire.
type DecryptedMessage struct {
	Sender  string `json:"s"`
	Content string `json:"c"` // Plaintext; local only
	Time    time.Time
}

// Presence status values carried by PresencePacket.Status.
const (
	PresenceActive = "active"
	PresenceAway   = "away"
)

// PresencePacket announces a user's presence status. It is plaintext
// metadata the server is meant to see: the server binds the nickname to the
// authenticated connection and fans the update out to peers, so the wire
// nickname is never trusted.
type PresencePacket struct {
	Nickname string `json:"n"`
	Status   string `json:"s"` // PresenceActive or PresenceAway
}

type JoinPacket struct {
	Nickname string `json:"n"`
	PubKey   []byte `json:"k"` // HPKE KEM public key for message encryption
}

type DirPacket struct {
	CurrentUsers map[string][]byte `json:"cu"` // map[Nickname]HPKEPublicKey
}

type LeavePacket struct {
	Nickname string `json:"n"`
	PubKey   []byte `json:"k"` // HPKE KEM public key of the departed user
}

type ErrPacket struct {
	Message string `json:"m"`
}

type PingPacket struct {
	Time time.Time
}

type PubKey struct {
	PubKey []byte `json:"k"`
}
