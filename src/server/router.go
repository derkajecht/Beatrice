package server

import (
	"log/slog"
	"net"

	"github.com/derkajecht/Beatrice/src/shared"
)

// HandleHandshake receives the handshake packet from the client
// validates username and public key, and saves the client to the hub
// returns an error if the handshake fails
func HandleHandshake(p shared.GeneralPacket) error {
	return nil
}

// HandleMessage handles incoming message packets - todo
func HandleMessage(p shared.MessagePacket, conn net.Conn) {
	slog.Debug("message received from client", "client", conn.RemoteAddr())
}

// HandleJoin handles join requests - todo
func HandleJoin(p shared.JoinPacket, conn net.Conn) {}

// HandleLeave handles leave requests - todo
func HandleLeave(p shared.LeavePacket, conn net.Conn) {}

// HandleError processes error packets and disconnects client on protocol mismatch
// func HandleError(p shared.ErrPacket, conn net.Conn) {
// 	slog.Debug("error packet received", "client", conn.RemoteAddr())
// 	switch p.Message {
// 	case "invalid_protocol_format":
// 		slog.Error("Critical server/client mismatch")
// 	default:
// 		slog.Debug("non-critical error handled: ", p.Message)
// 	}
// }

// HandleDir refreshes directory list - todo
func HandleDir(p shared.DirPacket, conn net.Conn) {}

// HandleChallenge processes authentication challenges - todo
func HandleChallenge(p shared.ChallengePacket, conn net.Conn) {}
