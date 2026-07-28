package server

import (
	"github.com/derkajecht/Beatrice/internal/shared"
)

// NewClientPacket creates a new client packet struct with the given nickname and public key
func NewClientPacket(nickname string, pubKey []byte) *shared.User {
	return &shared.User{
		Nickname: nickname,
		Crypto: shared.CryptoPacket{
			PubKey: shared.PubKey{
				PubKey: pubKey,
			},
		},
	}
}

// NewErrPacket creates a new error packet struct with the given error message
func NewErrPacket(errMsg string) *shared.ErrPacket {
	return &shared.ErrPacket{
		Message: errMsg,
	}
}

// NewDirPacket creates a new dir packet struct with the given user list
func NewDirPacket(GetUserList map[string][]byte) *shared.DirPacket {
	return &shared.DirPacket{
		CurrentUsers: GetUserList,
	}
}

// GetUserList returns a list of nicknames and their public keys for all connected users
// except the given nickname
func GetUserList(r *shared.Room, nickname string) map[string][]byte {
	r.Lock()
	defer r.Unlock()

	// create a new map to store the user list
	// and iterate over the clients map if the given nickname is not in the map
	// (user doesn't get their own information)
	userList := make(map[string][]byte)
	for _, client := range r.Clients {
		if client != nil && client.Nickname != nickname {
			userList[client.Nickname] = client.PubKey.PubKey
		}
	}
	return userList
}
