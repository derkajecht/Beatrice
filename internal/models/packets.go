package models

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
	Sender    string `json:"s,omitempty"`
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
