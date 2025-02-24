package storagetx

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/golem-base/storageutil"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/holiman/uint256"
)

//go:generate go run ../../rlp/rlpgen -type StorageTransaction -out gen_storage_transaction_rlp.go

// GolemBaseStorageEntityCreated is the event signature for entity creation logs.
var GolemBaseStorageEntityCreated = crypto.Keccak256Hash([]byte("GolemBaseStorageEntityCreated(uint256,uint256)"))

// GolemBaseStorageEntityDeleted is the event signature for entity deletion logs.
var GolemBaseStorageEntityDeleted = crypto.Keccak256Hash([]byte("GolemBaseStorageEntityDeleted(uint256)"))

// GolemBaseStorageEntityUpdated is the event signature for entity update logs.
var GolemBaseStorageEntityUpdated = crypto.Keccak256Hash([]byte("GolemBaseStorageEntityUpdated(uint256,uint256)"))

// StorageTransaction represents a transaction that can be applied to the storage layer.
// It contains a list of Create operations, a list of Update operations and a list of Delete operations.
//
// Semantics of the transaction operations are as follows:
//   - Create: adds new entities to the storage layer. Each entity has a TTL (number of blocks), a payload and a list of annotations. The Key of the entity is derived from the payload content, the transaction hash where the entity was created and the index of the create operation in the transaction.
//   - Update: updates existing entities. Each entity has a key, a TTL (number of blocks), a payload and a list of annotations. If the entity does not exist, the operation fails, failing the whole transaction.
//   - Delete: removes entities from the storage layer. If the entity does not exist, the operation fails, failing back the whole transaction.
//
// The transaction is atomic, meaning that all operations are applied or none are.
//
// Annotations are key-value pairs where the key is a string and the value is either a string or a number.
// The key-value pairs are used to build indexes and to query the storage layer.
// Same key can have both string and numeric annotation, but not multiple values of the same type.
type StorageTransaction struct {
	Create []Create      `json:"create"`
	Update []Update      `json:"update"`
	Delete []common.Hash `json:"delete"`
}

type Create struct {
	TTL                uint64                          `json:"ttl"`
	Payload            []byte                          `json:"payload"`
	StringAnnotations  []storageutil.StringAnnotation  `json:"stringAnnotations"`
	NumericAnnotations []storageutil.NumericAnnotation `json:"numericAnnotations"`
}

type Update struct {
	EntityKey          common.Hash                     `json:"entityKey"`
	TTL                uint64                          `json:"ttl"`
	Payload            []byte                          `json:"payload"`
	StringAnnotations  []storageutil.StringAnnotation  `json:"stringAnnotations"`
	NumericAnnotations []storageutil.NumericAnnotation `json:"numericAnnotations"`
}

func (tx *StorageTransaction) Run(blockNumber uint64, txHash common.Hash, access storageutil.StateAccess) ([]*types.Log, error) {
	logs := []*types.Log{}

	storeEntity := func(key common.Hash, ap *storageutil.ActivePayload, emitLogs bool) error {

		buf := new(bytes.Buffer)
		err := rlp.Encode(buf, ap)
		if err != nil {
			return fmt.Errorf("failed to encode active payload: %w", err)
		}

		storageutil.SetGolemDBState(access, key, buf.Bytes())
		expiresAtBlockNumberBig := uint256.NewInt(ap.ExpiresAtBlock)
		{

			// create the key for the list of entities that will expire at the given block number
			expiredEntityKey := crypto.Keccak256Hash([]byte("golemBaseExpiresAtBlock"), expiresAtBlockNumberBig.Bytes())
			err = storageutil.AppendToKeyList(access, expiredEntityKey, key)
			if err != nil {
				return fmt.Errorf("failed to append to key list: %w", err)
			}

		}

		if emitLogs {
			// create the log for the created entity
			log := &types.Log{
				Address:     common.Address{0x01}, // Set the appropriate address if needed
				Topics:      []common.Hash{GolemBaseStorageEntityCreated, key},
				Data:        expiresAtBlockNumberBig.Bytes(),
				BlockNumber: blockNumber,
			}
			logs = append(logs, log)
		}

		{
			for _, stringAnnotation := range ap.StringAnnotations {
				err = storageutil.AppendToKeyList(
					access,
					crypto.Keccak256Hash(
						[]byte("golemBaseStringAnnotation"),
						[]byte(stringAnnotation.Key),
						[]byte(stringAnnotation.Value),
					),
					key,
				)
				if err != nil {
					return fmt.Errorf("failed to append to key list: %w", err)
				}
			}

			for _, numericAnnotation := range ap.NumericAnnotations {
				err = storageutil.AppendToKeyList(
					access,
					crypto.Keccak256Hash(
						[]byte("golemBaseNumericAnnotation"),
						[]byte(numericAnnotation.Key),
						binary.BigEndian.AppendUint64(nil, numericAnnotation.Value),
					),
					key,
				)
				if err != nil {
					return fmt.Errorf("failed to append to key list: %w", err)
				}
			}
		}

		return nil

	}

	for i, create := range tx.Create {
		// Convert i to a big integer and pad to 32 bytes
		bigI := big.NewInt(int64(i))
		paddedI := common.LeftPadBytes(bigI.Bytes(), 32)

		key := crypto.Keccak256Hash(txHash.Bytes(), create.Payload, paddedI)

		ap := &storageutil.ActivePayload{
			ExpiresAtBlock:     blockNumber + create.TTL,
			Payload:            create.Payload,
			StringAnnotations:  create.StringAnnotations,
			NumericAnnotations: create.NumericAnnotations,
		}

		err := storeEntity(key, ap, true)

		if err != nil {
			return nil, err
		}

	}

	deleteEntity := func(toDelete common.Hash, emitLogs bool) error {
		v := storageutil.GetGolemDBState(access, toDelete)

		ap := storageutil.ActivePayload{}

		err := rlp.DecodeBytes(v, &ap)
		if err != nil {
			return fmt.Errorf("failed to decode active payload for %s: %w", toDelete.Hex(), err)
		}

		for _, stringAnnotation := range ap.StringAnnotations {
			err = storageutil.RemoveKeyFromList(
				access,
				crypto.Keccak256Hash(
					[]byte("golemBaseStringAnnotation"),
					[]byte(stringAnnotation.Key),
					[]byte(stringAnnotation.Value),
				),
				toDelete,
			)
			if err != nil {
				return fmt.Errorf("failed to remove key %s from the string annotation list: %w", toDelete, err)
			}
		}

		for _, numericAnnotation := range ap.NumericAnnotations {
			err = storageutil.RemoveKeyFromList(
				access,
				crypto.Keccak256Hash(
					[]byte("golemBaseNumericAnnotation"),
					[]byte(numericAnnotation.Key),
					binary.BigEndian.AppendUint64(nil, numericAnnotation.Value),
				),
				toDelete,
			)
			if err != nil {
				return fmt.Errorf("failed to remove key %s from the numeric annotation list: %w", toDelete, err)
			}
		}

		expiresAtBlockNumberBig := uint256.NewInt(ap.ExpiresAtBlock)

		// create the key for the list of entities that will expire at the given block number
		expiredEntityKey := crypto.Keccak256Hash([]byte("golemBaseExpiresAtBlock"), expiresAtBlockNumberBig.Bytes())

		err = storageutil.RemoveKeyFromList(access, expiredEntityKey, toDelete)
		if err != nil {
			return fmt.Errorf("failed to append to key list: %w", err)
		}

		storageutil.DeleteGolemDBState(access, toDelete)

		if emitLogs {

			// create the log for the created entity
			log := &types.Log{
				Address:     common.Address{0x01}, // Set the appropriate address if needed
				Topics:      []common.Hash{GolemBaseStorageEntityDeleted, toDelete},
				Data:        []byte{},
				BlockNumber: blockNumber,
			}

			logs = append(logs, log)
		}

		return nil

	}

	for _, toDelete := range tx.Delete {
		err := deleteEntity(toDelete, true)
		if err != nil {
			return nil, err
		}
	}

	for _, update := range tx.Update {
		err := deleteEntity(update.EntityKey, false)
		if err != nil {
			return nil, err
		}

		ap := &storageutil.ActivePayload{
			ExpiresAtBlock:     blockNumber + update.TTL,
			Payload:            update.Payload,
			StringAnnotations:  update.StringAnnotations,
			NumericAnnotations: update.NumericAnnotations,
		}

		err = storeEntity(update.EntityKey, ap, false)

		if err != nil {
			return nil, err
		}

		logs = append(logs, &types.Log{
			Address:     common.Address{0x01}, // Set the appropriate address if needed
			Topics:      []common.Hash{GolemBaseStorageEntityUpdated, update.EntityKey},
			Data:        common.BigToHash(big.NewInt(int64(ap.ExpiresAtBlock))).Bytes(),
			BlockNumber: blockNumber,
		})

	}

	// TODO: implement update

	return logs, nil
}

func ExecuteTransaction(d []byte, blockNumber uint64, txHash common.Hash, access storageutil.StateAccess) ([]*types.Log, error) {
	tx := &StorageTransaction{}
	err := rlp.DecodeBytes(d, tx)
	if err != nil {
		return nil, fmt.Errorf("failed to decode storage transaction: %w", err)
	}
	return tx.Run(blockNumber, txHash, access)
}
