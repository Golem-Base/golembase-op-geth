package entity

import (
	"fmt"

	"github.com/ethereum/go-ethereum/arkiv/compression"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/golem-base/storageutil"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity/entityexpiration"
)

type ExtendBTLResult struct {
	OldExpiresAtBlock uint64
	Owner             common.Address
	TotalBytes        uint64
}

func ExtendBTL(
	access storageutil.StateAccess,
	entityKey common.Hash,
	numberOfBlocks uint64,
) (*ExtendBTLResult, error) {

	ent, err := GetEntityMetaData(access, entityKey)
	if err != nil {
		return nil, err
	}

	err = entityexpiration.RemoveFromEntitiesToExpire(access, ent.ExpiresAtBlock, entityKey)
	if err != nil {
		return nil, fmt.Errorf("failed to remove from entities to expire at block %d: %w", ent.ExpiresAtBlock, err)
	}

	oldExpiresAtBlock := ent.ExpiresAtBlock

	ent.ExpiresAtBlock += numberOfBlocks

	err = entityexpiration.AddToEntitiesToExpireAtBlock(access, ent.ExpiresAtBlock, entityKey)
	if err != nil {
		return nil, fmt.Errorf("failed to add to entities to expire at block %d: %w", ent.ExpiresAtBlock, err)
	}

	entityMetaDataSize, err := StoreEntityMetaData(access, entityKey, *ent)
	if err != nil {
		return nil, fmt.Errorf("failed to store entity meta data: %w", err)
	}

	compressed := GetCompressedPayload(access, entityKey)
	decompressed, err := compression.BrotliDecompress(compressed)
	if err != nil {
		return nil, fmt.Errorf("failed to decompress payload for entity %s: %w", entityKey.Hex(), err)
	}

	return &ExtendBTLResult{
		OldExpiresAtBlock: oldExpiresAtBlock,
		Owner:             ent.Owner,
		TotalBytes:        entityMetaDataSize + uint64(len(decompressed)),
	}, nil

}
