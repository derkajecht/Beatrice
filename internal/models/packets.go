package models

import (
	"github.com/derkajecht/Beatrice/internal/types"
)

// NewErrPacket creates a new error packet struct with the given error message
func NewErrPacket(p *types.ErrPacket, errMsg string) (map[string]string, error) {
	// set values for error packet struct
	p = &types.ErrPacket{
		Type:    "e",
		Message: errMsg,
	}

	// convert error packet struct to map[string]string
	// and return the map and an error if any
	errPacketMap, err := types.ConvertPacketToMap(p)
	if err != nil {
		return nil, err
	}

	return errPacketMap, err
}

// NewDirPacket creates a new dir packet struct with the given user list
func NewDirPacket(p *types.DirPacket, GetUserList map[string]string) (map[string]string, error) {
	// set values for dir packet struct
	p = &types.DirPacket{
		Type:          "d",
		Current_Users: GetUserList,
	}

	// convert dir packet struct to map[string]string
	// and return the map and an error if any
	dirPacketMap, err := types.ConvertPacketToMap(p)
	if err != nil {
		return nil, err
	}

	return dirPacketMap, err
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
