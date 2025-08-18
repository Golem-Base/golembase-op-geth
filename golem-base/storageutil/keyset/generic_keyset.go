package keyset

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/golem-base/storageutil"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/keyset/hashmap"
	"github.com/holiman/uint256"
)

type StateAccess = storageutil.StateAccess

type Array[T any] interface {
	Size() *uint256.Int
	Get(index *uint256.Int) (T, error)
	Append(value T)
	RemoveLast() error
	Set(index *uint256.Int, value T) error
	Iterate(yield func(T) bool)
	Clear()
}

type StorageImpl[T any] interface {
	NewArray(db storageutil.StateAccess, address common.Hash) Array[T]
	ValueToHash(value T) common.Hash
}

var zeroHash = common.Hash{}
var MapKeyPrefix = []byte("golemBase.keyset.map")

// GenericContainsValue checks if the given value exists in the set identified by setKey.
// It returns true if the value is present in the set, false otherwise.
func GenericContainsValue[T any](db StateAccess, impl StorageImpl[T], setKey common.Hash, value T) bool {
	m := hashmap.NewMap(db, MapKeyPrefix, setKey[:])
	return m.Get(impl.ValueToHash(value)) != zeroHash
}

// GenericAddValue adds a value to the set identified by setKey.
// If the value already exists in the set, it does nothing.
// Returns an error if there are any issues during the operation.
func GenericAddValue[T any](db StateAccess, impl StorageImpl[T], setKey common.Hash, value T) error {

	array := impl.NewArray(db, setKey)
	m := hashmap.NewMap(db, MapKeyPrefix, setKey[:])

	// if the value is already in the set, do nothing
	if ContainsValue(db, setKey, impl.ValueToHash(value)) {
		return nil
	}

	array.Append(value)
	m.Set(impl.ValueToHash(value), array.Size().Bytes32())

	return nil
}

// GenericRemoveValue removes a value from the set identified by setKey.
// It does nothing if the value is not in the set.
// For non-empty sets, it moves the last element to the position of the removed element
// to maintain a compact array representation.
// Returns an error if there are any issues during the operation.
func GenericRemoveValue[T any](db StateAccess, impl StorageImpl[T], setKey common.Hash, value T) error {

	array := impl.NewArray(db, setKey)
	m := hashmap.NewMap(db, MapKeyPrefix, setKey[:])

	if !ContainsValue(db, setKey, impl.ValueToHash(value)) {
		return nil
	}

	elementIndex := new(uint256.Int).SetBytes32(m.Get(impl.ValueToHash(value)).Bytes())
	elementIndex.SubUint64(elementIndex, 1)

	oldSize := array.Size()

	lastElementIndex := new(uint256.Int).Set(oldSize)
	lastElementIndex.SubUint64(lastElementIndex, 1)
	lastElementValue, err := array.Get(lastElementIndex)
	if err != nil {
		return fmt.Errorf("failed to get last element: %w", err)
	}

	m.Set(impl.ValueToHash(value), zeroHash)

	if lastElementIndex.Cmp(elementIndex) != 0 {
		array.Set(elementIndex, lastElementValue)
		elementIndexPlusOne := new(uint256.Int).Set(elementIndex)
		elementIndexPlusOne.AddUint64(elementIndexPlusOne, 1)
		m.Set(impl.ValueToHash(lastElementValue), elementIndexPlusOne.Bytes32())
	}

	err = array.RemoveLast()
	if err != nil {
		return fmt.Errorf("failed to remove last element: %w", err)
	}

	return nil

}

// GenericSize returns the number of elements in the set as a uint256
func GenericSize[T any](db StateAccess, impl StorageImpl[T], setKey common.Hash) *uint256.Int {
	array := impl.NewArray(db, setKey)
	return array.Size()
}

// GenericClear removes all elements from the set.
// It iterates through all values in the set and clears their mappings,
// then resets the set's size to zero.
// This operation is O(n) where n is the number of elements in the set.
func GenericClear[T any](db StateAccess, impl StorageImpl[T], setKey common.Hash) {
	array := impl.NewArray(db, setKey)
	m := hashmap.NewMap(db, MapKeyPrefix, setKey[:])

	for v := range array.Iterate {
		m.Set(impl.ValueToHash(v), zeroHash)
	}
	array.Clear()
}

func GenericIterate[T any](db StateAccess, impl StorageImpl[T], setKey common.Hash) func(func(T) bool) {
	array := impl.NewArray(db, setKey)
	return array.Iterate
}
