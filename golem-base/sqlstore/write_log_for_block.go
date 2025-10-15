package sqlstore

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/golem-base/address"
	"github.com/ethereum/go-ethereum/golem-base/storagetx"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/holiman/uint256"
)

func WriteLogForBlockSqlite(
	sqlStore *SQLStore,
	db state.Database,
	hc *core.HeaderChain,
	block *types.Block,
	chainID *big.Int,
	receipts []*types.Receipt,
) (blockDbHash common.Hash, err error) {

	ctx := context.Background()

	defer func() {
		if err != nil {
			log.Error("failed to write log for block", "block", block.NumberU64(), "error", err)
		}
	}()

	networkID := chainID.String()

	SyncSQLiteFromChain(block.NumberU64()-1, block.ParentHash(), hc.GetHeaderByHash(block.ParentHash()).Root, sqlStore, chainID, db)

	txns := block.Transactions()

	signer := types.LatestSignerForChainID(chainID)

	wal := BlockWal{
		BlockInfo: BlockInfo{
			Number:     block.NumberU64(),
			Hash:       block.Hash(),
			ParentHash: block.ParentHash(),
		},
		Operations: []Operation{},
	}

	for i, tx := range txns {
		receipt := receipts[i]
		if receipt.Status == types.ReceiptStatusFailed {
			continue
		}

		toAddr := common.Address{}
		if tx.To() != nil {
			toAddr = *tx.To()
		}

		switch {
		case tx.Type() == types.DepositTxType:
			for _, l := range receipt.Logs {
				if len(l.Topics) != 2 {
					continue
				}

				if l.Topics[0] != storagetx.GolemBaseStorageEntityDeleted {
					continue
				}

				key := l.Topics[1]

				wal.Operations = append(wal.Operations, Operation{
					Delete: &key,
				})

			}
			// create
		case toAddr == address.GolemBaseStorageProcessorAddress:

			stx := storagetx.StorageTransaction{}
			err := rlp.DecodeBytes(tx.Data(), &stx)
			if err != nil {
				return common.Hash{}, fmt.Errorf("failed to decode storage transaction: %w", err)
			}

			createdLogs := []*types.Log{}
			updatedLogs := []*types.Log{}
			extendedLogs := []*types.Log{}

			for _, log := range receipt.Logs {
				if len(log.Topics) < 2 {
					continue
				}

				if log.Topics[0] == storagetx.GolemBaseStorageEntityCreated {
					createdLogs = append(createdLogs, log)
				}

				if log.Topics[0] == storagetx.GolemBaseStorageEntityUpdated {
					updatedLogs = append(updatedLogs, log)
				}

				if log.Topics[0] == storagetx.GolemBaseStorageEntityBTLExtended {
					extendedLogs = append(extendedLogs, log)
				}

			}

			for i, create := range stx.Create {

				l := createdLogs[i]
				key := l.Topics[1]
				expiresAtBlockU256 := uint256.NewInt(0).SetBytes(l.Data)
				expiresAtBlock := expiresAtBlockU256.Uint64()

				from, err := types.Sender(signer, tx)
				if err != nil {
					return common.Hash{}, fmt.Errorf("failed to get sender of create transaction %s: %w", tx.Hash().Hex(), err)
				}

				cr := Create{
					EntityKey:          key,
					ExpiresAtBlock:     expiresAtBlock,
					Payload:            create.Payload,
					StringAnnotations:  create.StringAnnotations,
					NumericAnnotations: create.NumericAnnotations,
					Owner:              from,
				}

				wal.Operations = append(wal.Operations, Operation{
					Create: &cr,
				})

			}

			for _, del := range stx.Delete {
				wal.Operations = append(wal.Operations, Operation{
					Delete: &del,
				})
			}

			for i, update := range stx.Update {

				log := updatedLogs[i]
				key := log.Topics[1]
				expiresAtBlockU256 := uint256.NewInt(0).SetBytes(log.Data)
				expiresAtBlock := expiresAtBlockU256.Uint64()

				ur := Update{
					EntityKey:          key,
					ExpiresAtBlock:     expiresAtBlock,
					Payload:            update.Payload,
					StringAnnotations:  update.StringAnnotations,
					NumericAnnotations: update.NumericAnnotations,
				}

				wal.Operations = append(wal.Operations, Operation{
					Update: &ur,
				})
			}

			for i, extend := range stx.Extend {

				log := extendedLogs[i]

				oldExpiresAtU256 := uint256.NewInt(0).SetBytes(log.Data[:32])
				oldExpiresAt := oldExpiresAtU256.Uint64()

				newExpiresAtU256 := uint256.NewInt(0).SetBytes(log.Data[32:])
				newExpiresAt := newExpiresAtU256.Uint64()

				ex := ExtendBTL{
					EntityKey:    extend.EntityKey,
					OldExpiresAt: oldExpiresAt,
					NewExpiresAt: newExpiresAt,
				}

				wal.Operations = append(wal.Operations, Operation{
					Extend: &ex,
				})
			}

		default:
		}

	}

	blockDbHash, err = sqlStore.InsertBlock(
		ctx,
		wal,
		networkID,
	)

	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to insert block: %w", err)
	}

	return blockDbHash, nil
}
