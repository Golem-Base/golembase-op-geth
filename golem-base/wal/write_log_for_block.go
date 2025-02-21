package wal

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/golem-base/storagetx"
	"github.com/ethereum/go-ethereum/golem-base/storageutil"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/holiman/uint256"
)

type Operation struct {
	Create *Create      `json:"create,omitempty"`
	Update *Update      `json:"update,omitempty"`
	Delete *common.Hash `json:"delete,omitempty"`
}

type Create struct {
	EntityKey          common.Hash                     `json:"entityKey"`
	ExpiresAtBlock     uint64                          `json:"expiresAtBlock"`
	Payload            []byte                          `json:"payload"`
	StringAnnotations  []storageutil.StringAnnotation  `json:"stringAnnotations"`
	NumericAnnotations []storageutil.NumericAnnotation `json:"numericAnnotations"`
}

type Update struct {
	EntityKey          common.Hash                     `json:"entityKey"`
	ExpiresAtBlock     uint64                          `json:"expiresAtBlock"`
	Payload            []byte                          `json:"payload"`
	StringAnnotations  []storageutil.StringAnnotation  `json:"stringAnnotations"`
	NumericAnnotations []storageutil.NumericAnnotation `json:"numericAnnotations"`
}

func WriteLogForBlock(dir string, block *types.Block, receipts []*types.Receipt) (err error) {

	defer func() {
		if err != nil {
			log.Error("failed to write log for block", "block", block.NumberU64(), "error", err)
		}
	}()

	tempFilename := fmt.Sprintf("block-%020d.json.temp", block.NumberU64())
	finalFilename := fmt.Sprintf("block-%020d.json", block.NumberU64())

	tf, err := os.OpenFile(filepath.Join(dir, tempFilename), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to open temp file: %w", err)
	}
	defer func() {
		tf.Close()
		os.Remove(filepath.Join(dir, tempFilename))
	}()

	enc := json.NewEncoder(tf)

	txns := block.Transactions()

	for i, tx := range txns {
		receipt := receipts[i]
		if receipt.Status == types.ReceiptStatusFailed {
			continue
		}

		switch tx.Type() {
		case types.GolemBaseHousekeepingTxType:
			for _, l := range receipt.Logs {
				if len(l.Topics) != 2 {
					continue
				}

				if l.Topics[0] != storagetx.GolemBaseStorageEntityDeleted {
					continue
				}

				key := l.Topics[1]

				err := enc.Encode(Operation{
					Delete: &key,
				})
				if err != nil {
					return fmt.Errorf("failed to encode delete operation: %w", err)
				}

			}
			// create
		case types.GolemBaseUpdateStorageTxType:

			stx := storagetx.StorageTransaction{}
			err := rlp.DecodeBytes(tx.Data(), &stx)
			if err != nil {
				return fmt.Errorf("failed to decode storage transaction: %w", err)
			}

			createdLogs := []*types.Log{}
			updatedLogs := []*types.Log{}

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

			}

			for i, create := range stx.Create {

				l := createdLogs[i]
				key := l.Topics[1]
				expiresAtBlockU256 := uint256.NewInt(0).SetBytes(l.Data)
				expiresAtBlock := expiresAtBlockU256.Uint64()

				cr := Create{
					EntityKey:          key,
					ExpiresAtBlock:     expiresAtBlock,
					Payload:            create.Payload,
					StringAnnotations:  create.StringAnnotations,
					NumericAnnotations: create.NumericAnnotations,
				}

				err := enc.Encode(Operation{
					Create: &cr,
				})
				if err != nil {
					return fmt.Errorf("failed to encode create operation: %w", err)
				}

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

				err := enc.Encode(Operation{
					Update: &ur,
				})
				if err != nil {
					return fmt.Errorf("failed to encode update operation: %w", err)
				}
			}

			for _, del := range stx.Delete {
				err := enc.Encode(Operation{
					Delete: &del,
				})
				if err != nil {
					return fmt.Errorf("failed to encode delete operation: %w", err)
				}
			}

		default:
		}

	}

	err = tf.Close()
	if err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	err = os.Rename(filepath.Join(dir, tempFilename), filepath.Join(dir, finalFilename))
	if err != nil {
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}
