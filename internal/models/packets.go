package models

import (
	"github.com/derkajecht/Beatrice/internal/types"
)

// NewErrPacket creates a new error packet struct with the given error message
func NewErrPacket(errMsg string) *types.ErrPacket {
	return &types.ErrPacket{
		Message: errMsg,
	}
}

// NewDirPacket creates a new dir packet struct with the given user list
func NewDirPacket(GetUserList map[string]string) *types.DirPacket {
	return &types.DirPacket{
		CurrentUsers: GetUserList,
	}
}

// GetUserList returns a list of nicknames and their public keys for all connected users
// except the given nickname
func GetUserList(r *types.Room, nickname string) map[string]string {
	r.Lock()
	defer r.Unlock()

	// create a new map to store the user list
	// and iterate over the clients map if the given nickname is not in the map
	// (user doesn't get their own information)
	userList := make(map[string]string)
	for _, client := range r.Clients {
		if client != nil && client.Nickname != nickname {
			userList[client.Nickname] = client.PubKey
		}
	}
	return userList
}
