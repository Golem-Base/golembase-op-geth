package entity

import (
	"fmt"
	"regexp"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/golem-base/storageutil"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity/allentities"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity/annotationindex"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity/entitiesofowner"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity/entityexpiration"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/keyset"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/numericalannotationindex"
)

// This regex should not allow $ as the first character, since we use that for
// special meta-annotations like $owner.
const AnnotationIdentRegex string = `[\p{L}_][\p{L}\p{N}_]*`

var annotationIdentRegexCompiled *regexp.Regexp = regexp.MustCompile(fmt.Sprintf("^%s$", AnnotationIdentRegex))

type StateAccess = storageutil.StateAccess

func Store(
	access StateAccess,
	txHash common.Hash,
	key common.Hash,
	sender common.Address,
	emd EntityMetaData,
	payload []byte,
) error {

	// Validate the annotation identifiers
	for _, annotation := range emd.StringAnnotations {
		if !annotationIdentRegexCompiled.MatchString(annotation.Key) {
			return fmt.Errorf("Invalid annotation identifier (must match `%s`): %s",
				annotationIdentRegexCompiled.String(),
				annotation.Key,
			)
		}
	}
	for _, annotation := range emd.NumericAnnotations {
		if !annotationIdentRegexCompiled.MatchString(annotation.Key) {
			return fmt.Errorf("Invalid annotation identifier (must match `%s`): %s",
				annotationIdentRegexCompiled.String(),
				annotation.Key,
			)
		}
	}

	err := allentities.AddEntity(access, key)
	if err != nil {
		return fmt.Errorf("failed to add entity to all entities: %w", err)
	}

	err = entitiesofowner.AddEntity(access, sender, key)
	if err != nil {
		return fmt.Errorf("failed to add entity to owner entities: %w", err)
	}

	err = StoreEntityMetaData(access, key, emd)
	if err != nil {
		return fmt.Errorf("failed to store entity meta data: %w", err)
	}

	err = entityexpiration.AddToEntitiesToExpireAtBlock(access, emd.ExpiresAtBlock, key)
	if err != nil {
		return fmt.Errorf("failed to add entity to entities to expire: %w", err)
	}

	for _, stringAnnotation := range emd.StringAnnotations {
		err = keyset.AddValue(
			access,
			annotationindex.StringAnnotationIndexKey(stringAnnotation.Key, stringAnnotation.Value),
			key,
		)
		if err != nil {
			return fmt.Errorf("failed to append to key list: %w", err)
		}
	}

	for _, numericAnnotation := range emd.NumericAnnotations {
		// We pass the tx hash as the seed for the RNG, this ensures that the underlying
		// treap is deterministic
		ix := numericalannotationindex.New(access, numericAnnotation.Key, txHash)
		err := ix.Add(numericAnnotation.Value, key)

		if err != nil {
			return fmt.Errorf("failed to append entity to annotation index: %w", err)
		}
	}

	StorePayload(access, key, payload)

	return nil
}
