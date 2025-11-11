package entity

import (
	"bytes"
	"fmt"

	"github.com/ethereum/go-ethereum/arkiv/compression"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/stateblob"
	"github.com/ethereum/go-ethereum/rlp"
)

func StoreEntityMetaData(access StateAccess, key common.Hash, emd EntityMetaData) (uint64, error) {
	hash := crypto.Keccak256Hash(EntityMetaDataSalt, key[:])

	buf := new(bytes.Buffer)
	err := rlp.Encode(buf, &emd)
	if err != nil {
		return 0, fmt.Errorf("failed to encode entity meta data: %w", err)
	}

	bytes := buf.Bytes()

	compressed := compression.MustBrotliCompress(bytes)

	stateblob.SetBlob(access, hash, compressed)
	return uint64(len(bytes)), nil
}
