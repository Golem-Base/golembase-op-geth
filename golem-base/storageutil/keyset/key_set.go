// Package keyset provides a set data structure implementation for the Ethereum state.
// This is a Go implementation of the same data structure pattern used in OpenZeppelin's
// EnumerableSet (https://github.com/OpenZeppelin/openzeppelin-contracts/blob/master/contracts/utils/structs/EnumerableSet.sol)
// It provides O(1) operations for adding, removing, and checking membership in a set,
// while also maintaining the ability to enumerate elements.
// ContainsValue checks if the given value exists in the set identified by setKey.
// It returns true if the value is present in the set, false otherwise.
package keyset

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/golem-base/storageutil"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/keyset/array"
	"github.com/holiman/uint256"
)

type Impl struct{}

func (Impl) NewArray(db storageutil.StateAccess, address common.Hash) Array[common.Hash] {
	return array.NewArray(db, address)
}

func (Impl) ValueToHash(value common.Hash) common.Hash {
	return value
}

var impl StorageImpl[common.Hash] = Impl{}

// ContainsValue checks if the given value exists in the set identified by setKey.
// It returns true if the value is present in the set, false otherwise.
func ContainsValue(db StateAccess, setKey common.Hash, value common.Hash) bool {
	return GenericContainsValue(db, impl, setKey, value)
}

// AddValue adds a value to the set identified by setKey.
// If the value already exists in the set, it does nothing.
// Returns an error if there are any issues during the operation.
func AddValue(db StateAccess, setKey common.Hash, value common.Hash) error {
	return GenericAddValue(db, impl, setKey, value)
}

// RemoveValue removes a value from the set identified by setKey.
// It does nothing if the value is not in the set.
// For non-empty sets, it moves the last element to the position of the removed element
// to maintain a compact array representation.
// Returns an error if there are any issues during the operation.
func RemoveValue(db StateAccess, setKey common.Hash, value common.Hash) error {
	return GenericRemoveValue(db, impl, setKey, value)
}

// Size returns the number of elements in the set as a uint256
func Size(db StateAccess, setKey common.Hash) *uint256.Int {
	return GenericSize(db, impl, setKey)
}

// Clear removes all elements from the set.
// It iterates through all values in the set and clears their mappings,
// then resets the set's size to zero.
// This operation is O(n) where n is the number of elements in the set.
func Clear(db StateAccess, setKey common.Hash) {
	GenericClear(db, impl, setKey)
}

func Iterate(db StateAccess, setKey common.Hash) func(yield func(value common.Hash) bool) {
	return GenericIterate(db, impl, setKey)
}
