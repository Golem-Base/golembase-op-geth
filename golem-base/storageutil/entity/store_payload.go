package entity

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/stateblob"
)

var PayloadSalt = []byte("arkivPayload")

func StorePayload(access StateAccess, key common.Hash, payload []byte) {
	hash := crypto.Keccak256Hash(PayloadSalt, key[:])
	stateblob.SetBlob(access, hash, payload)
}
