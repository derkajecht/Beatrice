package server

import "github.com/derkajecht/Beatrice/internal/shared"

// GetPubKey returns the public key from the handshake packet
func GetPubKey(p shared.HandshakePacket) string {
	return string(p.PubKey.PubKey)
}
