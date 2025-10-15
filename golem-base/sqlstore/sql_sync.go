package sqlstore

import (
	"context"
	"fmt"
	"math/big"

	"encoding/hex"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/golem-base/hasher"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity/allentities"
	"github.com/ethereum/go-ethereum/log"
)

func SyncSQLiteFromChain(blockNumber uint64, blockHash common.Hash, blockRoot common.Hash, sqlStore *SQLStore, chainID *big.Int, db state.Database) error {
	ctx := context.Background()

	log.Info("Checking SQLite sync status", "chainHead", blockNumber, "chainHeadHash", blockHash.Hex())

	// Check SQLite processing status
	processingStatus, err := sqlStore.GetProcessingStatus(ctx, chainID.String())
	if err != nil {
		return fmt.Errorf("failed to get processing status: %w", err)
	}

	var haveToResync bool
	switch {
	case processingStatus.LastProcessedBlockNumber == 0 && blockNumber != 0:
		haveToResync = true
	case processingStatus.LastProcessedBlockNumber != int64(blockNumber):
		haveToResync = true
	case processingStatus.LastProcessedBlockHash != blockHash.Hex():
		haveToResync = true
	default:
		haveToResync = false
	}

	if !haveToResync {
		log.Info("SQLite already in sync", "block", blockNumber)
		return nil
	}

	log.Info("SQLite needs sync",
		"sqliteBlock", processingStatus.LastProcessedBlockNumber,
		"sqliteHash", processingStatus.LastProcessedBlockHash,
		"chainBlock", blockNumber,
		"chainHash", blockHash.Hex())

	// Create entity iterator from current state
	entityIterator := func(
		yield func(*struct {
			Key      common.Hash
			Metadata entity.EntityMetaData
			Payload  []byte
		}, error) bool,
	) {
		statedb, err := state.New(blockRoot, db)
		if err != nil {
			yield(nil, fmt.Errorf("failed to get statedb: %w", err))
			return
		}

		log.Info("Starting entity iteration from chain state")

		for entityKey := range allentities.Iterate(statedb) {
			log.Info("Iterating over entity", "entityKey", entityKey.Hex())
			emd, err := entity.GetEntityMetaData(statedb, entityKey)
			if err != nil {
				yield(nil, fmt.Errorf("failed to get entity metadata for key %s: %w", entityKey.Hex(), err))
				return
			}
			payload := entity.GetPayload(statedb, entityKey)

			if !yield(&struct {
				Key      common.Hash
				Metadata entity.EntityMetaData
				Payload  []byte
			}{
				Key:      entityKey,
				Metadata: *emd,
				Payload:  payload,
			}, nil) {
				return
			}
		}
	}

	// Perform the snap sync
	err = sqlStore.SnapSyncToBlock(ctx, chainID.String(), blockNumber, blockHash, entityIterator)
	if err != nil {
		return fmt.Errorf("failed to snap sync to block: %w", err)
	}

	hasher.ProcessEvents(sqlStore.fuseDriver.GetEvents(true), sqlStore.hasher)
	hasherRoot := sqlStore.hasher.Root()
	log.Info("Hasher after sync", hex.EncodeToString(hasherRoot.Bytes()))

	sqlStore.hasher.Build()
	hasherRoot = sqlStore.hasher.Root()
	log.Info("Hasher after synd and build", hex.EncodeToString(hasherRoot.Bytes()))

	log.Info("Initial sync completed", "block", blockNumber, "hash", blockHash.Hex())
	return nil
}
