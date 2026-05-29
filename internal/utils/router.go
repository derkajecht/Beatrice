package utils

import (
	"log"
	"net"

	"github.com/derkajecht/Beatrice/internal/models"
)

func HandleHandshake(p models.HandshakePacket, conn net.Conn) (bool, *models.ErrPacket, *models.SuccessPacket) {
	// TODO: Implement handshake logic
	// client should send a message packet with the following data:
	// {"t":"h", "n": nickname,	"k": pubkey}
	// server should send a handshake packet to the client containing a success message
	// and all connected users

	// check if user already connected
	if models.ChatRoom.Clients[conn] != nil {
		err_packet := models.NewErrPacket("User already connected")
		return false, err_packet, nil
	}

	// check if nickname and public key are not empty
	if p.Nickname == "" || p.PubKey == "" {
		err_packet := models.NewErrPacket("Nickname and/or public key are empty")
		return false, err_packet, nil
	}

	// check if user is trying to connect with their own nickname
	if p.Nickname == models.ChatRoom.Clients[conn].Nickname {
		err_packet := models.NewErrPacket("Nickname cannot be the same as your own")
		return false, err_packet, nil
	}

	// add user to the chat room
	models.ChatRoom.Lock()
	models.ChatRoom.Clients[conn] = &models.Client{
		Conn:     conn,
		Nickname: p.Nickname,
		PubKey:   p.PubKey,
	}
	models.ChatRoom.Unlock()

	// send dir packet to client (nickname, public key)
	list := models.ChatRoom.GetUserList(p.Nickname)
	dir_packet := models.NewDirPacket(list)

	err := SendToClient(conn, dir_packet.Message)
	if err != nil {
		log.Println("Error sending dir packet:", err)
		return false, nil, nil
	}

	return true, nil, nil
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
