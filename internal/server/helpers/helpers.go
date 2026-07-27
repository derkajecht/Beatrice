package helpers

import "github.com/derkajecht/Beatrice/internal/shared/types"

// GetPubKey returns the public key from the handshake packet
func GetPubKey(p types.HandshakePacket) string {
	return string(p.PubKey.PubKey)
}
