package housekeepingtx

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/golem-base/address"
	"github.com/ethereum/go-ethereum/golem-base/storagetx"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/keyset"
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

		err := entity.Delete(db, toDelete)
		if err != nil {
			return fmt.Errorf("failed to delete entity: %w", err)
		}

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
