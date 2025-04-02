package housekeepingtx

import (
	"encoding/binary"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/golem-base/address"
	"github.com/ethereum/go-ethereum/golem-base/storagetx"
	"github.com/ethereum/go-ethereum/golem-base/storageutil"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/allentities"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entitiesofowner"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/keyset"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/stateblob"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/holiman/uint256"
)

func ExecuteTransaction(blockNumber uint64, txHash common.Hash, db vm.StateDB) ([]*types.Log, error) {

	// create the golem base storage processor address if it doesn't exist
	// this is needed to be able to use the state access interface
	if !db.Exist(address.GolemBaseStorageProcessorAddress) {
		db.CreateAccount(address.GolemBaseStorageProcessorAddress)
		db.CreateContract(address.GolemBaseStorageProcessorAddress)
		db.SetNonce(address.GolemBaseStorageProcessorAddress, 1, tracing.NonceChangeNewContract)
	}

	logs := []*types.Log{}

	deleteEntity := func(toDelete common.Hash) error {
		v := stateblob.GetBlob(db, toDelete)

		ap := storageutil.ActivePayload{}

		err := allentities.RemoveEntity(db, toDelete)
		if err != nil {
			return fmt.Errorf("failed to remove entity from all entities: %w", err)
		}

		err = rlp.DecodeBytes(v, &ap)
		if err != nil {
			return fmt.Errorf("failed to decode active payload for %s: %w", toDelete.Hex(), err)
		}

		for _, stringAnnotation := range ap.StringAnnotations {
			err = keyset.RemoveValue(
				db,
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
			err = keyset.RemoveValue(
				db,
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

		err = keyset.RemoveValue(db, expiredEntityKey, toDelete)
		if err != nil {
			return fmt.Errorf("failed to append to key list: %w", err)
		}

		err = entitiesofowner.RemoveEntity(db, ap.Owner, toDelete)
		if err != nil {
			return fmt.Errorf("failed to remove entity from owner entities: %w", err)
		}

		stateblob.DeleteBlob(db, toDelete)

		// create the log for the created entity
		log := &types.Log{
			Address:     address.GolemBaseStorageProcessorAddress, // Set the appropriate address if needed
			Topics:      []common.Hash{storagetx.GolemBaseStorageEntityDeleted, toDelete},
			Data:        []byte{},
			BlockNumber: blockNumber,
		}

		logs = append(logs, log)

		return nil

	}

	expiresAtBlockNumberBig := uint256.NewInt(blockNumber)

	entitiesToExpireForBlockKey := crypto.Keccak256Hash([]byte("golemBaseExpiresAtBlock"), expiresAtBlockNumberBig.Bytes())

	for key := range keyset.Iterator(db, entitiesToExpireForBlockKey) {
		err := deleteEntity(key)
		if err != nil {
			return nil, fmt.Errorf("failed to delete entity %s: %w", key.Hex(), err)
		}
	}

	keyset.Clear(db, entitiesToExpireForBlockKey)

	return logs, nil
}
