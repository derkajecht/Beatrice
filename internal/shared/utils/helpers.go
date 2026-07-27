// Package utils provides utility functions for the server and client
package utils

import (
	"github.com/derkajecht/Beatrice/internal/shared/types"
)

type User struct {
	*types.User
}

// GetPublicKey returns the public key of the user
func (u *User) GetPublicKey() []byte {
	return u.Crypto.PubKey.PubKey
}

// GetNickname returns the nickname of the user
func (u *User) GetNickname() string {
	return u.Nickname
}
