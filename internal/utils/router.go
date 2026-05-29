package utils

import (
	"net"

	"github.com/derkajecht/Beatrice/internal/models"
)

func HandleHandshake(p models.HandshakePacket, conn net.Conn) {
	// TODO: Implement handshake logic
}

func HandleMessage(p models.MessagePacket, conn net.Conn) {
	// TODO: Implement message logic
}

func HandleJoin(p models.JoinPacket, conn net.Conn) {
	// TODO: Implement join logic
}

func HandleLeave(p models.LeavePacket, conn net.Conn) {
	// TODO: Implement leave logic
}

func HandleError(p models.ErrPacket, conn net.Conn) {
	// TODO: Implement error logic
}

func HandleDir(p models.DirPacket, conn net.Conn) {
	// TODO: Implement dir logic
}

func HandleChallenge(p models.ChallengePacket, conn net.Conn) {
	// TODO: Implement challenge logic
}
