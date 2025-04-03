package entity

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity/allentities"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity/annotationindex"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity/entitiesofowner"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/keyset"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/stateblob"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/holiman/uint256"
)

func Delete(access StateAccess, toDelete common.Hash) error {
	err := allentities.RemoveEntity(access, toDelete)
	if err != nil {
		return fmt.Errorf("failed to remove entity from all entities: %w", err)
	}

	v := stateblob.GetBlob(access, toDelete)

	ap := ActivePayload{}

	err = rlp.DecodeBytes(v, &ap)
	if err != nil {
		return fmt.Errorf("failed to decode active payload for %s: %w", toDelete.Hex(), err)
	}

	for _, stringAnnotation := range ap.StringAnnotations {
		setKey := annotationindex.StringAnnotationIndexKey(stringAnnotation.Key, stringAnnotation.Value)
		err := keyset.RemoveValue(
			access,
			setKey,
			toDelete,
		)
		if err != nil {
			return fmt.Errorf("failed to remove key %s from the string annotation list: %w", toDelete, err)
		}

	}

	for _, numericAnnotation := range ap.NumericAnnotations {
		setKeys := annotationindex.NumericAnnotationIndexKey(numericAnnotation.Key, numericAnnotation.Value)
		err := keyset.RemoveValue(
			access,
			setKeys,
			toDelete,
		)
		if err != nil {
			return fmt.Errorf("failed to remove key %s from the numeric annotation list: %w", toDelete, err)
		}
	}

	expiresAtBlockNumberBig := uint256.NewInt(ap.ExpiresAtBlock)

	// create the key for the list of entities that will expire at the given block number
	expiredEntityKey := crypto.Keccak256Hash([]byte("golemBaseExpiresAtBlock"), expiresAtBlockNumberBig.Bytes())

	err = keyset.RemoveValue(access, expiredEntityKey, toDelete)
	if err != nil {
		return fmt.Errorf("failed to append to key list: %w", err)
	}

	err = entitiesofowner.RemoveEntity(access, ap.Owner, toDelete)
	if err != nil {
		return fmt.Errorf("failed to remove entity from owner entities: %w", err)
	}

	stateblob.DeleteBlob(access, toDelete)

	return nil
}
