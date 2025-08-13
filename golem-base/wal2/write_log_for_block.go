package wal2

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/golem-base/address"
	"github.com/ethereum/go-ethereum/golem-base/etl/sqlite/etl"
	"github.com/ethereum/go-ethereum/golem-base/storagetx"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/holiman/uint256"
)

type BlockInfo struct {
	Number     uint64      `json:"number,string"`
	Hash       common.Hash `json:"hash"`
	ParentHash common.Hash `json:"parentHash"`
}

type Operation struct {
	Create *Create      `json:"create,omitempty"`
	Update *Update      `json:"update,omitempty"`
	Delete *common.Hash `json:"delete,omitempty"`
	Extend *ExtendBTL   `json:"extend,omitempty"`
}

type Create struct {
	EntityKey          common.Hash                `json:"entityKey"`
	ExpiresAtBlock     uint64                     `json:"expiresAtBlock"`
	Payload            []byte                     `json:"payload"`
	StringAnnotations  []entity.StringAnnotation  `json:"stringAnnotations"`
	NumericAnnotations []entity.NumericAnnotation `json:"numericAnnotations"`
	Owner              common.Address             `json:"owner"`
}

type Update struct {
	EntityKey          common.Hash                `json:"entityKey"`
	ExpiresAtBlock     uint64                     `json:"expiresAtBlock"`
	Payload            []byte                     `json:"payload"`
	StringAnnotations  []entity.StringAnnotation  `json:"stringAnnotations"`
	NumericAnnotations []entity.NumericAnnotation `json:"numericAnnotations"`
}

type ExtendBTL struct {
	EntityKey    common.Hash `json:"entityKey"`
	OldExpiresAt uint64      `json:"oldExpiresAt"`
	NewExpiresAt uint64      `json:"newExpiresAt"`
}

func WriteLogForBlockSqlite(
	sqliteETL *etl.ETL,
	block *types.Block,
	chainID *big.Int,
	receipts []*types.Receipt,
) (err error) {

	defer func() {
		if err != nil {
			log.Error("failed to write log for block", "block", block.NumberU64(), "error", err)
		}
	}()

	// enc.Encode(BlockInfo{
	// 	Number:     block.NumberU64(),
	// 	Hash:       block.Hash(),
	// 	ParentHash: block.ParentHash(),
	// })

	// sqliteETL.Begin()
	// defer sqliteETL.

	// sqliteETL.InsertBlock(context.Background(), block.NumberU64(), block.Hash(), block.ParentHash())

	txns := block.Transactions()

	signer := types.LatestSignerForChainID(chainID)

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

				key = key

				// TODO insert delete operation into sqlite
				// err := enc.Encode(Operation{
				// 	Delete: &key,
				// })
				// if err != nil {
				// 	return fmt.Errorf("failed to encode delete operation: %w", err)
				// }

			}
			// create
		case toAddr == address.GolemBaseStorageProcessorAddress:

			stx := storagetx.StorageTransaction{}
			err := rlp.DecodeBytes(tx.Data(), &stx)
			if err != nil {
				return fmt.Errorf("failed to decode storage transaction: %w", err)
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
					return fmt.Errorf("failed to get sender of create transaction %s: %w", tx.Hash().Hex(), err)
				}

				cr := Create{
					EntityKey:          key,
					ExpiresAtBlock:     expiresAtBlock,
					Payload:            create.Payload,
					StringAnnotations:  create.StringAnnotations,
					NumericAnnotations: create.NumericAnnotations,
					Owner:              from,
				}

				cr = cr

				// TODO insert create operation into sqlite
				// err = enc.Encode(Operation{
				// 	Create: &cr,
				// })
				// if err != nil {
				// 	return fmt.Errorf("failed to encode create operation: %w", err)
				// }

			}

			for _, del := range stx.Delete {
				del = del
				// TODO insert delete operation into sqlite
				// err := enc.Encode(Operation{
				// 	Delete: &del,
				// })
				// if err != nil {
				// 	return fmt.Errorf("failed to encode delete operation: %w", err)
				// }
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

				ur = ur

				// TODO insert update operation into sqlite
				// err := enc.Encode(Operation{
				// 	Update: &ur,
				// })
				// if err != nil {
				// 	return fmt.Errorf("failed to encode update operation: %w", err)
				// }
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

				ex = ex

				// TODO insert extend operation into sqlite
				// err := enc.Encode(Operation{
				// 	Extend: &ex,
				// })
				// if err != nil {
				// 	return fmt.Errorf("failed to encode extend operation: %w", err)
				// }
			}

		default:
		}

	}

	return nil
}
