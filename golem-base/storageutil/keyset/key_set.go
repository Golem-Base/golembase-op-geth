// Package keyset provides a set data structure implementation for the Ethereum state.
// This is a Go implementation of the same data structure pattern used in OpenZeppelin's
// EnumerableSet (https://github.com/OpenZeppelin/openzeppelin-contracts/blob/master/contracts/utils/structs/EnumerableSet.sol)
// It provides O(1) operations for adding, removing, and checking membership in a set,
// while also maintaining the ability to enumerate elements.
package keyset

import (
	"errors"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/golem-base/storageutil"
	"github.com/holiman/uint256"
)

type StateAccess = storageutil.StateAccess

var zeroHash = common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000000")
var oneUint256 = new(uint256.Int).SetUint64(1)

// ContainsValue checks if the given value exists in the set identified by setKey.
// It returns true if the value is present in the set, false otherwise.
func ContainsValue(db StateAccess, setKey common.Hash, value common.Hash) bool {
	mapKey := crypto.Keccak256Hash([]byte("golemBase.keyset.map"), setKey[:], value[:])
	valueIndex := db.GetState(storageutil.GolemDBAddress, mapKey)
	return valueIndex != zeroHash
}

// AddValue adds a value to the set identified by setKey.
// If the value already exists in the set, it does nothing.
// Returns an error if there are any issues during the operation.
func AddValue(db StateAccess, setKey common.Hash, value common.Hash) error {

	// if the value is already in the set, do nothing
	if ContainsValue(db, setKey, value) {
		return nil
	}

	// get the current length of the set
	lenValue := db.GetState(storageutil.GolemDBAddress, setKey)
	lenValueInt := new(uint256.Int).SetBytes(lenValue[:])

	lenValueInt.AddUint64(lenValueInt, 1)

	// store the new length
	db.SetState(storageutil.GolemDBAddress, setKey, lenValueInt.Bytes32())

	mapKey := crypto.Keccak256Hash([]byte("golemBase.keyset.map"), setKey[:], value[:])

	mapValue := db.GetState(storageutil.GolemDBAddress, mapKey)
	if mapValue != zeroHash {
		return errors.New("map value is not zero")
	}

	// store the new value index+1 in the map
	db.SetState(storageutil.GolemDBAddress, mapKey, lenValueInt.Bytes32())

	listKeyInt := new(uint256.Int).SetBytes32(setKey[:])
	listKeyInt.Add(listKeyInt, lenValueInt)

	oldValue := db.GetState(storageutil.GolemDBAddress, listKeyInt.Bytes32())
	if oldValue != zeroHash {
		return errors.New("new entry in the list is not zero in storage")
	}

	db.SetState(storageutil.GolemDBAddress, listKeyInt.Bytes32(), value)

	return nil

}

func nextHash(h common.Hash) common.Hash {
	v := new(uint256.Int).SetBytes32(h[:])
	v.Add(v, oneUint256)
	return common.Hash(v.Bytes32())
}

// RemoveValue removes a value from the set identified by setKey.
// It does nothing if the value is not in the set.
// For non-empty sets, it moves the last element to the position of the removed element
// to maintain a compact array representation.
// Returns an error if there are any issues during the operation.
func RemoveValue(db StateAccess, setKey common.Hash, value common.Hash) error {

	// if the value is not in the set, do nothing
	if !ContainsValue(db, setKey, value) {
		return nil
	}

	arrayLenAsHash := db.GetState(storageutil.GolemDBAddress, setKey)

	arrayLen := new(uint256.Int).SetBytes(arrayLenAsHash[:])

	mapKey := crypto.Keccak256Hash([]byte("golemBase.keyset.map"), setKey[:], value[:])

	// if the set is empty, set the map and the set to zero
	if arrayLen.Cmp(oneUint256) == 0 {
		db.SetState(storageutil.GolemDBAddress, mapKey, zeroHash)
		db.SetState(storageutil.GolemDBAddress, setKey, zeroHash)
		db.SetState(storageutil.GolemDBAddress, nextHash(setKey), zeroHash)
		return nil
	}

	// get the index of the value in the array
	arrayIndexAsHash := db.GetState(storageutil.GolemDBAddress, mapKey)
	arrayIndex := new(uint256.Int).SetBytes(arrayIndexAsHash[:])

	// if the index is out of bounds, this should never happen
	if arrayLen.Cmp(arrayIndex) < 0 {
		return errors.New("value index is out of bounds, this should never happen")
	}

	// clear the mapping for the value
	db.SetState(storageutil.GolemDBAddress, mapKey, zeroHash)

	// get the address of the value to remove
	toRemoveAddress := new(uint256.Int).SetBytes32(setKey[:])
	toRemoveAddress.Add(toRemoveAddress, arrayIndex)

	// get the address of the last element in the array
	lastElementAddress := new(uint256.Int).SetBytes32(setKey[:])
	lastElementAddress.Add(lastElementAddress, arrayLen)
	lastElementValue := db.GetState(storageutil.GolemDBAddress, lastElementAddress.Bytes32())

	// store the last element in the place of the value to remove
	db.SetState(storageutil.GolemDBAddress, toRemoveAddress.Bytes32(), lastElementValue)

	// decrement the length of the array
	arrayLen.SubUint64(arrayLen, 1)
	db.SetState(storageutil.GolemDBAddress, setKey, arrayLen.Bytes32())

	// update the mapping for the last element
	lastElementMapKey := crypto.Keccak256Hash([]byte("golemBase.keyset.map"), setKey[:], lastElementValue[:])
	db.SetState(storageutil.GolemDBAddress, lastElementMapKey, arrayIndex.Bytes32())

	// clear last slot in the array
	db.SetState(storageutil.GolemDBAddress, lastElementAddress.Bytes32(), zeroHash)

	return nil

}

// Size returns the number of elements in the set as a uint256
func Size(db StateAccess, setKey common.Hash) *uint256.Int {
	lenValue := db.GetState(storageutil.GolemDBAddress, setKey)
	return new(uint256.Int).SetBytes(lenValue[:])
}

// Clear removes all elements from the set.
// It iterates through all values in the set and clears their mappings,
// then resets the set's size to zero.
// This operation is O(n) where n is the number of elements in the set.
func Clear(db StateAccess, setKey common.Hash) {
	// Get the current size of the set
	arrayLen := Size(db, setKey)

	// If the set is already empty, do nothing
	if arrayLen.IsZero() {
		return
	}

	// For each element in the set
	for i := new(uint256.Int).SetUint64(1); i.Cmp(arrayLen) <= 0; i.AddUint64(i, 1) {
		// Get the element address
		elementAddress := new(uint256.Int).SetBytes32(setKey[:])
		elementAddress.Add(elementAddress, i)

		// Get the element value
		value := db.GetState(storageutil.GolemDBAddress, elementAddress.Bytes32())

		// Clear the mapping for this value
		mapKey := crypto.Keccak256Hash([]byte("golemBase.keyset.map"), setKey[:], value[:])
		db.SetState(storageutil.GolemDBAddress, mapKey, zeroHash)

		// Clear the element storage
		db.SetState(storageutil.GolemDBAddress, elementAddress.Bytes32(), zeroHash)
	}

	// Reset the set size to zero
	db.SetState(storageutil.GolemDBAddress, setKey, zeroHash)
}

func Iterate(db StateAccess, setKey common.Hash) func(yield func(value common.Hash) bool) {
	return func(yield func(value common.Hash) bool) {
		// Get the current size of the set
		arrayLen := Size(db, setKey)

		// If the set is empty, do nothing
		if arrayLen.IsZero() {
			return
		}

		// Iterate through each element in the set
		for i := new(uint256.Int).SetUint64(1); i.Cmp(arrayLen) <= 0; i.AddUint64(i, 1) {
			// Get the element address
			elementAddress := new(uint256.Int).SetBytes32(setKey[:])
			elementAddress.Add(elementAddress, i)

			// Get the element value
			value := db.GetState(storageutil.GolemDBAddress, elementAddress.Bytes32())

			// Apply the yield function to the value
			// If it returns false, stop iteration
			if !yield(value) {
				return
			}
		}
	}
}

// Iterator returns a channel that can be used with a range loop to iterate over the set values.
// This allows for more idiomatic iteration using 'for value := range keyset.Iterator(db, setKey) {}'.
// The channel is closed automatically when the iteration is complete.
func Iterator(db StateAccess, setKey common.Hash) <-chan common.Hash {
	values := make(chan common.Hash)

	go func() {
		defer close(values)

		// Get the current size of the set
		arrayLen := Size(db, setKey)

		// If the set is empty, return immediately (channel will be closed)
		if arrayLen.IsZero() {
			return
		}

		// Iterate through each element in the set
		for i := new(uint256.Int).SetUint64(1); i.Cmp(arrayLen) <= 0; i.AddUint64(i, 1) {
			// Get the element address
			elementAddress := new(uint256.Int).SetBytes32(setKey[:])
			elementAddress.Add(elementAddress, i)

			// Get the element value
			value := db.GetState(storageutil.GolemDBAddress, elementAddress.Bytes32())

			// Send the value to the channel
			values <- value
		}
	}()

	return values
}
