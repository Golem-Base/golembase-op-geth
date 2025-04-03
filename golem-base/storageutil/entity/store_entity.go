package entity

import (
	"bytes"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/golem-base/storageutil"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity/allentities"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity/annotationindex"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity/entitiesofowner"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity/entityexpiration"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/keyset"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/stateblob"
	"github.com/ethereum/go-ethereum/rlp"
)

type StateAccess = storageutil.StateAccess

//go:generate go run ../../../rlp/rlpgen -type Annotations -out gen_annotations_rlp.go
type Annotations struct {
	String  []StringAnnotation
	Numeric []NumericAnnotation
}

func Store(
	access StateAccess,
	key common.Hash,
	sender common.Address,
	ap ActivePayload,
) error {

	err := allentities.AddEntity(access, key)
	if err != nil {
		return fmt.Errorf("failed to add entity to all entities: %w", err)
	}

	err = entitiesofowner.AddEntity(access, sender, key)
	if err != nil {
		return fmt.Errorf("failed to add entity to owner entities: %w", err)
	}

	buf := new(bytes.Buffer)
	err = rlp.Encode(buf, &ap)
	if err != nil {
		return fmt.Errorf("failed to encode active payload: %w", err)
	}

	stateblob.SetBlob(access, key, buf.Bytes())

	err = entityexpiration.AddToEntitiesToExpireAtBlock(access, ap.ExpiresAtBlock, key)
	if err != nil {
		return fmt.Errorf("failed to add entity to entities to expire: %w", err)
	}

	for _, stringAnnotation := range ap.StringAnnotations {
		err = keyset.AddValue(
			access,
			annotationindex.StringAnnotationIndexKey(stringAnnotation.Key, stringAnnotation.Value),
			key,
		)
		if err != nil {
			return fmt.Errorf("failed to append to key list: %w", err)
		}
	}

	for _, numericAnnotation := range ap.NumericAnnotations {
		err = keyset.AddValue(
			access,
			annotationindex.NumericAnnotationIndexKey(numericAnnotation.Key, numericAnnotation.Value),
			key,
		)
		if err != nil {
			return fmt.Errorf("failed to append to key list: %w", err)
		}
	}

	return nil
}
