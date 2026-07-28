package router

import (
	"log/slog"
	"net"

	"github.com/coder/websocket"
	"github.com/derkajecht/Beatrice/internal/server/helpers"
	"github.com/derkajecht/Beatrice/internal/server/models"
	"github.com/derkajecht/Beatrice/internal/server/validation"
	sharedvalidation "github.com/derkajecht/Beatrice/internal/shared/sharedvalidation"
	"github.com/derkajecht/Beatrice/internal/shared/types"
	"github.com/derkajecht/Beatrice/internal/shared/utils"
)

// HandleHandshake validates and creates client session
func HandleHandshake(c *websocket.Conn, p types.User) bool {
	// NOTE: do i need to add any RLock() or RUnlock() here?

	if _, ok := websocket.Hub.clients[c]; ok {
		errPacket := models.NewErrPacket("err_user_already_connected")
		utils.SendPacketToClient(conn, "e", errPacket)
		slog.Error("user already connected", "client", conn.RemoteAddr())
		return false
	}

	if sharedvalidation.HasEmptyArgs(p.Nickname, helpers.GetPubKey(p)) {
		errPacket := models.NewErrPacket("err_nickname_or_pubkey_empty")
		utils.SendPacketToClient(conn, "e", errPacket)
		slog.Error("nickname or pubkey empty", "client", conn.RemoteAddr())
		return false
	}

	if !validation.IsValidUsername(p.Nickname) {
		errPacket := models.NewErrPacket("err_nickname_taken")
		utils.SendPacketToClient(conn, "e", errPacket)
		slog.Error("nickname already taken", "client", conn.RemoteAddr())
		return false
	}

	types.ChatRoom.AddClient(conn, p.Nickname, p.PubKey)
	list := models.GetUserList(types.ChatRoom, p.Nickname)
	currentUsers := models.NewDirPacket(list)

	if err := utils.SendPacketToClient(conn, "d", currentUsers); err != nil {
		slog.Error("error sending dir packet", "err", err)
		types.ChatRoom.RemoveClient(conn)
		return false
	}

	return true
}

// HandleMessage handles incoming message packets - todo
func HandleMessage(p types.MessagePacket, conn net.Conn) {
	slog.Debug("message received from client", "client", conn.RemoteAddr())
}

// HandleJoin handles join requests - todo
func HandleJoin(p types.JoinPacket, conn net.Conn) {}

// HandleLeave handles leave requests - todo
func HandleLeave(p types.LeavePacket, conn net.Conn) {}

// HandleError processes error packets and disconnects client on protocol mismatch
func HandleError(p types.ErrPacket, conn net.Conn) {
	slog.Debug("error packet received", "client", conn.RemoteAddr())
	switch p.Message {
	case "invalid_protocol_format":
		slog.Error("Critical server/client mismatch")
	default:
		slog.Debug("non-critical error handled: ", p.Message)
	}
}

// HandleDir refreshes directory list - todo
func HandleDir(p types.DirPacket, conn net.Conn) {}

// HandleChallenge processes authentication challenges - todo
func HandleChallenge(p types.ChallengePacket, conn net.Conn) {}
