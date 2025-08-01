package storagetx

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/golem-base/address"
	"github.com/ethereum/go-ethereum/golem-base/storageaccounting"
	"github.com/ethereum/go-ethereum/golem-base/storageutil"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/holiman/uint256"
)

//go:generate protoc --proto_path=proto --go_out=. --go_opt=paths=source_relative proto/storagetx/storage_transaction.proto
//go:generate go run ../../rlp/rlpgen -type StorageTransaction -out gen_storage_transaction_rlp.go

// GolemBaseStorageEntityCreated is the event signature for entity creation logs.
var GolemBaseStorageEntityCreated = crypto.Keccak256Hash([]byte("GolemBaseStorageEntityCreated(uint256,uint256)"))

// GolemBaseStorageEntityDeleted is the event signature for entity deletion logs.
var GolemBaseStorageEntityDeleted = crypto.Keccak256Hash([]byte("GolemBaseStorageEntityDeleted(uint256)"))

// GolemBaseStorageEntityUpdated is the event signature for entity update logs.
var GolemBaseStorageEntityUpdated = crypto.Keccak256Hash([]byte("GolemBaseStorageEntityUpdated(uint256,uint256)"))

// GolemBaseStorageEntityBTLExtended is the event signature for extending BTL of an entity.
var GolemBaseStorageEntityBTLExtended = crypto.Keccak256Hash([]byte("GolemBaseStorageEntityBTLExtended(uint256,uint256,uint256)"))

func (tx *StorageTransaction) Run(blockNumber uint64, txHash common.Hash, sender common.Address, access storageutil.StateAccess) (_ []*types.Log, err error) {

	defer func() {
		if err != nil {
			log.Error("failed to run storage transaction", "error", err)
		}
	}()

	logs := []*types.Log{}

	storeEntity := func(key common.Hash, ap *entity.EntityMetaData, payload []byte, emitLogs bool) error {

		err := entity.Store(access, key, sender, ap, payload)
		if err != nil {
			return fmt.Errorf("failed to store entity: %w", err)
		}

		if emitLogs {
			expiresAtBlockNumberBig := uint256.NewInt(ap.ExpiresAtBlock)

			data := make([]byte, 32)
			expiresAtBlockNumberBig.PutUint256(data[:32])

			// create the log for the created entity
			log := &types.Log{
				Address:     address.GolemBaseStorageProcessorAddress,
				Topics:      []common.Hash{GolemBaseStorageEntityCreated, key},
				Data:        data,
				BlockNumber: blockNumber,
			}
			logs = append(logs, log)
		}

		return nil

	}

	for i, create := range tx.Create {

		if create.BTL == 0 {
			return nil, fmt.Errorf("create BTL is 0 for create %d", i)
		}

		// Convert i to a big integer and pad to 32 bytes
		bigI := big.NewInt(int64(i))
		paddedI := common.LeftPadBytes(bigI.Bytes(), 32)

		key := crypto.Keccak256Hash(txHash.Bytes(), create.Payload, paddedI)

		ap := &entity.EntityMetaData{
			Owner:              sender.Bytes(),
			ExpiresAtBlock:     blockNumber + create.BTL,
			StringAnnotations:  create.StringAnnotations,
			NumericAnnotations: create.NumericAnnotations,
		}

		err := storeEntity(key, ap, create.Payload, true)

		if err != nil {
			return nil, err
		}

	}

	deleteEntity := func(toDelete common.Hash, emitLogs bool) error {

		err := entity.Delete(access, toDelete)
		if err != nil {
			return fmt.Errorf("failed to delete entity: %w", err)
		}

		if emitLogs {

			// create the log for the created entity
			log := &types.Log{
				Address:     address.GolemBaseStorageProcessorAddress,
				Topics:      []common.Hash{GolemBaseStorageEntityDeleted, toDelete},
				Data:        []byte{},
				BlockNumber: blockNumber,
			}

			logs = append(logs, log)
		}

		return nil

	}

	for _, toDelete := range tx.Delete {
		toDelete := common.Hash(toDelete.GetEntityKey())
		metaData, err := entity.GetEntityMetaData(access, toDelete)
		if err != nil {
			return nil, fmt.Errorf("failed to get entity meta data for delete %s: %w", toDelete.Hex(), err)
		}

		if common.BytesToAddress(metaData.Owner) != sender {
			return nil, fmt.Errorf("failed to delete entity %s: %s is not the owner", toDelete.Hex(), sender.Hex())
		}

		err = deleteEntity(toDelete, true)
		if err != nil {
			return nil, err
		}
	}

	for _, update := range tx.Update {
		entityKey := common.BytesToHash(update.EntityKey)
		if update.BTL == 0 {
			return nil, fmt.Errorf("update BTL is 0 for entity %s", entityKey.Hex())
		}

		oldMetaData, err := entity.GetEntityMetaData(access, entityKey)
		if err != nil {
			return nil, fmt.Errorf("failed to get entity meta data for update %s: %w", entityKey.Hex(), err)
		}

		if common.BytesToAddress(oldMetaData.Owner) != sender {
			return nil, fmt.Errorf("failed to update entity %s: %s is not the owner", entityKey.Hex(), sender.Hex())
		}

		err = deleteEntity(entityKey, false)
		if err != nil {
			return nil, err
		}

		ap := &entity.EntityMetaData{
			ExpiresAtBlock:     blockNumber + update.BTL,
			StringAnnotations:  update.StringAnnotations,
			NumericAnnotations: update.NumericAnnotations,
			Owner:              oldMetaData.Owner,
		}

		err = storeEntity(entityKey, ap, update.Payload, false)

		if err != nil {
			return nil, err
		}

		expiresAtBlockNumberBig := uint256.NewInt(ap.ExpiresAtBlock)
		data := make([]byte, 32)
		expiresAtBlockNumberBig.PutUint256(data[:32])

		logs = append(logs, &types.Log{
			Address:     address.GolemBaseStorageProcessorAddress,
			Topics:      []common.Hash{GolemBaseStorageEntityUpdated, entityKey},
			Data:        data,
			BlockNumber: blockNumber,
		})

	}

	for _, extend := range tx.Extend {
		entityKey := common.BytesToHash(extend.EntityKey)
		newExpiresAtBlock, err := entity.ExtendBTL(access, entityKey, extend.NumberOfBlocks)
		if err != nil {
			return nil, fmt.Errorf("failed to extend BTL of entity %s: %w", entityKey.Hex(), err)
		}

		oldExpiresAtBlock := newExpiresAtBlock - extend.NumberOfBlocks

		oldExpiresAtBlockBig := uint256.NewInt(oldExpiresAtBlock)
		newExpiresAtBlockBig := uint256.NewInt(newExpiresAtBlock)

		data := make([]byte, 64)

		oldExpiresAtBlockBig.PutUint256(data[:32])
		newExpiresAtBlockBig.PutUint256(data[32:])

		logs = append(logs, &types.Log{
			Address:     address.GolemBaseStorageProcessorAddress,
			Topics:      []common.Hash{GolemBaseStorageEntityBTLExtended, entityKey},
			Data:        data,
			BlockNumber: blockNumber,
		})
	}

	return logs, nil
}

func ExecuteTransaction(d []byte, blockNumber uint64, txHash common.Hash, sender common.Address, access storageutil.StateAccess) ([]*types.Log, error) {
	tx := &StorageTransaction{}
	err := rlp.DecodeBytes(d, tx)
	if err != nil {
		return nil, fmt.Errorf("failed to decode storage transaction: %w", err)
	}

	st := storageaccounting.NewSlotUsageCounter(access)

	logs, err := tx.Run(blockNumber, txHash, sender, st)
	if err != nil {
		log.Error("Failed to run storage transaction", "error", err)
		return nil, fmt.Errorf("failed to run storage transaction: %w", err)
	}

	st.UpdateUsedSlotsForGolemBase()

	return logs, nil
}
