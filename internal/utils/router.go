package utils

import (
	"log/slog"
	"net"

	"github.com/derkajecht/Beatrice/internal/models"
	"github.com/derkajecht/Beatrice/internal/types"
)

func HandleHandshake(p types.HandshakePacket, conn net.Conn) (bool, error) {
	// TODO: Implement handshake logic
	// client should send a message packet with the following data:
	// {"t":"h", "n": nickname,	"k": pubkey}
	// server should send a handshake packet to the client containing a success message
	// and all connected users

	// safely read and write to the ChatRoom map
	types.ChatRoom.Lock()
	defer types.ChatRoom.Unlock()

	// check if user already connected
	if types.ChatRoom.Clients[conn] != nil {
		// build error packet
		errPacket := models.NewErrPacket("err_user_already_connected")
		// send error packet to client
		// this should trigger the UI to display an error message and return the user to the lobby
		err := SendPacketToClient(conn, "e", errPacket)
		slog.Error("user already connected", "err", err, "client", conn.RemoteAddr())
		return false, nil
	}

	// check if nickname and public key are not empty
	if p.Nickname == "" || p.PubKey == "" {
		// build error packet
		errPacket := models.NewErrPacket("err_nickname_or_pubkey_empty")
		// send error packet to client
		// this should trigger the UI to display an error message and return the user to the lobby
		err := SendPacketToClient(conn, "e", errPacket)
		slog.Error("nickname or public key empty", "err", err, "client", conn.RemoteAddr())
		return false, nil
	}

	// check if nickname is already in use
	for _, client := range types.ChatRoom.Clients {
		if client.Nickname == p.Nickname {
			// build error packet
			errPacket := models.NewErrPacket("err_nickname_taken")
			// send error packet to client
			// this should trigger the UI to display an error message and return the user to the lobby
			err := SendPacketToClient(conn, "e", errPacket)
			slog.Error("nickname already taken", "err", err, "client", conn.RemoteAddr())
			return false, nil
		}
	}

	// add user to the chat room
	types.ChatRoom.Clients[conn] = &types.Client{
		Conn:     conn,
		Nickname: p.Nickname,
		PubKey:   p.PubKey,
	}

	// get user list of all connected users except the new user
	list := models.GetUserList(types.ChatRoom, p.Nickname)

	// create dir packet
	currentUsers := models.NewDirPacket(list)

	// send dir packet to client
	err := SendPacketToClient(conn, "d", currentUsers)
	if err != nil {
		slog.Error("error sending dir packet", "err", err, "client", conn.RemoteAddr())
		delete(types.ChatRoom.Clients, conn)
		return false, nil
	}
	return true, nil
}

func HandleMessage(p types.MessagePacket, conn net.Conn) {
	// TODO: Implement message logic
}

func HandleJoin(p types.JoinPacket, conn net.Conn) {
	// TODO: Implement join logic
}

func HandleLeave(p types.LeavePacket, conn net.Conn) {
	// TODO: Implement leave logic
}

func HandleError(p types.ErrPacket, conn net.Conn) {
	// TODO: Implement error logic
	// the client should receive a message packet with the following data:
	// {"t":"e", "m": message}

	switch p.Message {
	case "invalid_protocol_format":
		// TODO: Implement invalid protocol format logic
		slog.Error("Critical client/server mismatch. Disconnecting")
		// call Disconnect/Quit function with the client
		DisconnectAndQuit(conn)

	case "err_nickname_taken":
		// TODO: Implement nickname taken logic
		// trigger UI to display error message "That nickname is already taken. Please choose another one."
		// trigger func to allow the user to choose another nickname
		// EnableNicknameChoice(conn)

	case "err_user_already_connected":
		// TODO: Implement user already connected logic
		// trigger UI to display error message "That nickname is already taken. Please choose another one."
		// trigger func to allow the user to choose another nickname
		// EnableNicknameChoice(conn)

	case "err_room_full":
		// TODO: Implement room full logic
		// trigger UI to display error message "The room is full. Please try again later."
		// ReturnToLobby(conn)

	case "err_connection_closed":
		// TODO: Implement connection closed logic
		// trigger UI to display error message "Connection closed. Please try again later."
		// ReturnToLobby(conn)

	case "err_invalid_pubkey":
		// TODO: Implement invalid pubkey logic
		// trigger UI to display error message "Invalid public key. Please try again later."
		// ReturnToLobby(conn)
	}

}

func HandleDir(p types.DirPacket, conn net.Conn) {
	// TODO: Implement dir logic
}

func HandleChallenge(p types.ChallengePacket, conn net.Conn) {
	// TODO: Implement challenge logic
}
