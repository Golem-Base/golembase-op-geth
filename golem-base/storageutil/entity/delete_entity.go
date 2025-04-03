package entity

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity/allentities"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity/annotationindex"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity/entitiesofowner"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity/entityexpiration"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/keyset"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/stateblob"
	"github.com/ethereum/go-ethereum/rlp"
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

	err = entityexpiration.RemoveFromEntitiesToExpire(access, ap.ExpiresAtBlock, toDelete)
	if err != nil {
		return fmt.Errorf("failed to remove entity from entities to expire: %w", err)
	}

	err = entitiesofowner.RemoveEntity(access, ap.Owner, toDelete)
	if err != nil {
		return fmt.Errorf("failed to remove entity from owner entities: %w", err)
	}

	stateblob.DeleteBlob(access, toDelete)

	return nil
}
