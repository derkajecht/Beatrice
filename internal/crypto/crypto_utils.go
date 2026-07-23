package crypto

import (
	"crypto/hpke"

	"github.com/derkajecht/Beatrice/internal/types"
)

type SuiteConfig struct {
	KEM  hpke.KEM
	KDF  hpke.KDF
	AEAD hpke.AEAD
	Info []byte
}

func NewCryptoSuite() *SuiteConfig {
	return &SuiteConfig{
		KEM:  hpke.MLKEM768X25519(),
		KDF:  hpke.HKDFSHA512(),
		AEAD: hpke.AES256GCM(),
		Info: []byte("Beatrice"),
	}
}

func NewUserSession() (*types.PubKey, *types.PrivKey) {
	// init suite algos
	suite := NewCryptoSuite()

	// derive a new key pair
	privKey, err := suite.KEM.GenerateKey()
	if err != nil {
		// TODO: might not want to panic here, leaving for now
		panic(err)
	}
	pubKey := privKey.PublicKey()

	// serialize the keys
	pubBytes := pubKey.Bytes()
	privBytes, err := privKey.Bytes()

	// store inside structs
	return &types.PubKey{PubKey: pubBytes}, &types.PrivKey{PrivKey: privBytes}
}
