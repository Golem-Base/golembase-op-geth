package entity

import (
	"io"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/stateblob"
)

func GetPayload(access StateAccess, key common.Hash) []byte {
	hash := crypto.Keccak256Hash(PayloadSalt, key[:])
	return stateblob.GetBlob(access, hash)
}

func GetPayloadSize(access StateAccess, key common.Hash) uint64 {
	hash := crypto.Keccak256Hash(PayloadSalt, key[:])
	return stateblob.GetBlobSize(access, hash)
}

func WritePayloadTo(access StateAccess, key common.Hash, w io.Writer) error {
	hash := crypto.Keccak256Hash(PayloadSalt, key[:])
	return stateblob.WriteTo(access, hash, w)
}
