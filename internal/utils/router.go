package utils

import (
	"log"
	"net"

	"github.com/derkajecht/Beatrice/internal/models"
	"github.com/derkajecht/Beatrice/internal/types"
)

func HandleHandshake(p types.HandshakePacket, conn net.Conn) (bool, map[string]string, error) {
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
		err_packet, err := models.NewErrPacket(&types.ErrPacket{}, "User already connected")
		if err != nil {
			log.Println("Error creating error packet:", err)
			return false, err_packet, nil
		}
		return false, err_packet, err
	}

	// check if nickname and public key are not empty
	if p.Nickname == "" || p.PubKey == "" {
		err_packet, err := models.NewErrPacket(&types.ErrPacket{}, "Nickname and/or public key are empty")
		if err != nil {
			log.Println("Error creating error packet:", err)
			return false, err_packet, nil
		}
		return false, err_packet, nil
	}

	// check if nickname is already in use
	for _, client := range types.ChatRoom.Clients {
		if client.Nickname == p.Nickname {
			err_packet, err := models.NewErrPacket(&types.ErrPacket{}, "Nickname already in use")
			if err != nil {
				log.Println("Error creating error packet:", err)
				return false, err_packet, nil
			}
			return false, err_packet, nil
		}
		return false, nil, nil
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
	userDirPacket, err := models.NewDirPacket(&types.DirPacket{}, list)
	if err != nil {
		log.Println("Error creating dir packet:", err)
		delete(types.ChatRoom.Clients, conn)
		return false, nil, nil
	}

	// send dir packet to client
	err = SendPacketToClient(conn, userDirPacket)
	if err != nil {
		log.Println("Error sending dir packet:", err)
		delete(types.ChatRoom.Clients, conn)
		return false, nil, nil
	}

	return true, nil, nil
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
}

func HandleDir(p types.DirPacket, conn net.Conn) {
	// TODO: Implement dir logic
}

func HandleChallenge(p types.ChallengePacket, conn net.Conn) {
	// TODO: Implement challenge logic
}
