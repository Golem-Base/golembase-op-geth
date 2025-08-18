package bytekeyset_test

import (
	"fmt"
	"slices"
	"sort"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/golem-base/storageutil"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/bytekeyset"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockStateAccess implements StateAccess interface for testing
type mockStateAccess struct {
	storage map[common.Address]map[common.Hash]common.Hash
}

func newMockStateAccess() *mockStateAccess {
	return &mockStateAccess{
		storage: make(map[common.Address]map[common.Hash]common.Hash),
	}
}

func (m *mockStateAccess) GetState(addr common.Address, key common.Hash) common.Hash {
	if _, exists := m.storage[addr]; !exists {
		return common.Hash{}
	}
	if val, exists := m.storage[addr][key]; exists {
		return val
	}
	return common.Hash{}
}

func (m *mockStateAccess) SetState(addr common.Address, key common.Hash, value common.Hash) common.Hash {
	zeroHash := common.Hash{}

	// If value is zero, delete the entry instead of storing it
	if value == zeroHash {
		if storageMap, exists := m.storage[addr]; exists {
			delete(storageMap, key)

			// If address map is now empty, delete it too
			if len(storageMap) == 0 {
				delete(m.storage, addr)
			}
		}
		return zeroHash
	}

	// Otherwise store the non-zero value
	if _, exists := m.storage[addr]; !exists {
		m.storage[addr] = make(map[common.Hash]common.Hash)
	}
	m.storage[addr][key] = value
	return value
}

// Helper method to get the number of entries in storage for testing
func (m *mockStateAccess) GetStorageEntryCount(addr common.Address) int {
	if storageMap, exists := m.storage[addr]; exists {
		return len(storageMap)
	}
	return 0
}

func (m *mockStateAccess) Print(addr common.Address) {
	keys := []common.Hash{}

	for key := range m.storage[addr] {
		keys = append(keys, key)
	}

	sort.Slice(keys, func(i, j int) bool {
		return keys[i].Big().Cmp(keys[j].Big()) < 0
	})

	for _, key := range keys {
		value := m.storage[addr][key]
		fmt.Printf("%s: %s\n", key.Hex(), value.Hex())

	}
}

// Helper function to create test values
func newHash(val string) common.Hash {
	return common.HexToHash(val)
}

func TestAddAndCheckValueInEmptySet(t *testing.T) {
	db := newMockStateAccess()
	setKey := newHash("0x1")
	value := byte(2)

	// Initially should not contain the value
	assert.False(t, bytekeyset.ContainsValue(db, setKey, value))

	// Add value
	err := bytekeyset.AddValue(db, setKey, value)
	assert.NoError(t, err)

	// Should contain the value after adding
	assert.True(t, bytekeyset.ContainsValue(db, setKey, value))
}

func TestAddDuplicateValue(t *testing.T) {
	db := newMockStateAccess()
	setKey := newHash("0x1")
	value := byte(2)

	// Add value first time
	err := bytekeyset.AddValue(db, setKey, value)
	assert.NoError(t, err)

	// Add same value second time
	err = bytekeyset.AddValue(db, setKey, value)
	assert.NoError(t, err)

	// Should still contain the value
	assert.True(t, bytekeyset.ContainsValue(db, setKey, value))
}

func TestRemoveValueFromSet(t *testing.T) {
	db := newMockStateAccess()
	setKey := newHash("0x1")
	value := byte(2)

	// Add and verify value
	err := bytekeyset.AddValue(db, setKey, value)
	assert.NoError(t, err)
	assert.True(t, bytekeyset.ContainsValue(db, setKey, value))

	// Remove value
	err = bytekeyset.RemoveValue(db, setKey, value)
	assert.NoError(t, err)

	// Should not contain the value after removal
	assert.False(t, bytekeyset.ContainsValue(db, setKey, value))
}

func TestRemoveNonExistentValue(t *testing.T) {
	db := newMockStateAccess()
	setKey := newHash("0x1")
	value := byte(2)

	// Try to remove value that was never added
	err := bytekeyset.RemoveValue(db, setKey, value)
	assert.NoError(t, err)
	assert.False(t, bytekeyset.ContainsValue(db, setKey, value))
}

func TestMultipleValuesInSet(t *testing.T) {
	db := newMockStateAccess()
	setKey := newHash("0x1")
	value1 := byte(2)
	value2 := byte(3)
	value3 := byte(4)

	// Add multiple values
	err := bytekeyset.AddValue(db, setKey, value1)
	assert.NoError(t, err)

	require.Equal(t, bytekeyset.Size(db, setKey).Uint64(), uint64(1))

	err = bytekeyset.AddValue(db, setKey, value2)
	assert.NoError(t, err)

	require.Equal(t, bytekeyset.Size(db, setKey).Uint64(), uint64(2))

	err = bytekeyset.AddValue(db, setKey, value3)
	assert.NoError(t, err)

	require.Equal(t, bytekeyset.Size(db, setKey).Uint64(), uint64(3))

	// Verify all values are in the set
	assert.True(t, bytekeyset.ContainsValue(db, setKey, value1))
	assert.True(t, bytekeyset.ContainsValue(db, setKey, value2))
	assert.True(t, bytekeyset.ContainsValue(db, setKey, value3))

	// Remove middle value
	err = bytekeyset.RemoveValue(db, setKey, value2)
	assert.NoError(t, err)

	// Verify state after removal
	assert.True(t, bytekeyset.ContainsValue(db, setKey, value1))
	assert.False(t, bytekeyset.ContainsValue(db, setKey, value2))
	assert.True(t, bytekeyset.ContainsValue(db, setKey, value3))

	value4 := byte(5)
	err = bytekeyset.AddValue(db, setKey, value4)
	assert.NoError(t, err)

	assert.True(t, bytekeyset.ContainsValue(db, setKey, value1))
	assert.False(t, bytekeyset.ContainsValue(db, setKey, value2))
	assert.True(t, bytekeyset.ContainsValue(db, setKey, value3))
	assert.True(t, bytekeyset.ContainsValue(db, setKey, value4))
}

func TestSizeEmptySet(t *testing.T) {
	db := newMockStateAccess()
	setKey := newHash("0x1")

	// Empty set should have size 0
	size := bytekeyset.Size(db, setKey)
	assert.Equal(t, uint64(0), size.Uint64())
}

func TestSizeAfterAddingValues(t *testing.T) {
	db := newMockStateAccess()
	setKey := newHash("0x1")

	// Initially empty
	size := bytekeyset.Size(db, setKey)
	assert.Equal(t, uint64(0), size.Uint64())

	// Add one value
	value1 := byte(2)
	err := bytekeyset.AddValue(db, setKey, value1)
	assert.NoError(t, err)

	// keyset.Size should be 1
	size = bytekeyset.Size(db, setKey)
	assert.Equal(t, uint64(1), size.Uint64())

	// Add another value
	value2 := byte(3)
	err = bytekeyset.AddValue(db, setKey, value2)
	assert.NoError(t, err)

	// keyset.Size should be 2
	size = bytekeyset.Size(db, setKey)
	assert.Equal(t, uint64(2), size.Uint64())
}

func TestSizeAfterRemovingValues(t *testing.T) {
	db := newMockStateAccess()
	setKey := newHash("0x1")

	// Add two values
	value1 := byte(2)
	value2 := byte(3)

	err := bytekeyset.AddValue(db, setKey, value1)
	assert.NoError(t, err)

	err = bytekeyset.AddValue(db, setKey, value2)
	assert.NoError(t, err)

	// keyset.Size should be 2
	size := bytekeyset.Size(db, setKey)
	assert.Equal(t, uint64(2), size.Uint64())

	// Remove one value
	err = bytekeyset.RemoveValue(db, setKey, value1)
	assert.NoError(t, err)

	// keyset.Size should be 1
	size = bytekeyset.Size(db, setKey)
	assert.Equal(t, uint64(1), size.Uint64())

	// Remove another value
	err = bytekeyset.RemoveValue(db, setKey, value2)
	assert.NoError(t, err)

	// keyset.Size should be 0
	size = bytekeyset.Size(db, setKey)
	assert.Equal(t, uint64(0), size.Uint64())
}

func TestSizeWithDuplicateValues(t *testing.T) {
	db := newMockStateAccess()
	setKey := newHash("0x1")
	value := byte(2)

	// Initially empty
	size := bytekeyset.Size(db, setKey)
	assert.Equal(t, uint64(0), size.Uint64())

	// Add value
	err := bytekeyset.AddValue(db, setKey, value)
	assert.NoError(t, err)

	// keyset.Size should be 1
	size = bytekeyset.Size(db, setKey)
	assert.Equal(t, uint64(1), size.Uint64())

	// Add same value again
	err = bytekeyset.AddValue(db, setKey, value)
	assert.NoError(t, err)

	// keyset.Size should still be 1 (no duplicates)
	size = bytekeyset.Size(db, setKey)
	assert.Equal(t, uint64(1), size.Uint64())
}

func TestClearEmptySet(t *testing.T) {
	db := newMockStateAccess()
	setKey := newHash("0x1")

	// Clear an empty set
	bytekeyset.Clear(db, setKey)

	// Size should still be 0
	size := bytekeyset.Size(db, setKey)
	assert.Equal(t, uint64(0), size.Uint64())

	// Storage should be empty
	assert.Equal(t, 0, db.GetStorageEntryCount(storageutil.GolemDBAddress))
}

func TestClearSetWithSingleValue(t *testing.T) {
	db := newMockStateAccess()
	setKey := newHash("0x1")
	value := byte(2)

	// Add a single value
	err := bytekeyset.AddValue(db, setKey, value)
	assert.NoError(t, err)

	// Verify the value was added
	assert.True(t, bytekeyset.ContainsValue(db, setKey, value))
	size := bytekeyset.Size(db, setKey)
	assert.Equal(t, uint64(1), size.Uint64())

	// Storage should have entries
	entriesBefore := db.GetStorageEntryCount(storageutil.GolemDBAddress)
	assert.Greater(t, entriesBefore, 0)

	// Clear the set
	bytekeyset.Clear(db, setKey)

	// Verify the set is empty
	assert.False(t, bytekeyset.ContainsValue(db, setKey, value))
	size = bytekeyset.Size(db, setKey)
	assert.Equal(t, uint64(0), size.Uint64())

	// Storage should be completely empty after clearing
	assert.Equal(t, 0, db.GetStorageEntryCount(storageutil.GolemDBAddress))
}

func TestClearSetWithMultipleValues(t *testing.T) {
	db := newMockStateAccess()
	setKey := newHash("0x1")
	values := []byte{
		byte(2),
		byte(3),
		byte(4),
		byte(5),
		byte(6),
	}

	// Add multiple values
	for _, value := range values {
		err := bytekeyset.AddValue(db, setKey, value)
		assert.NoError(t, err)
		assert.True(t, bytekeyset.ContainsValue(db, setKey, value))
	}

	// Verify the set size
	size := bytekeyset.Size(db, setKey)
	assert.Equal(t, uint64(len(values)), size.Uint64())

	// Storage should have entries
	entriesBefore := db.GetStorageEntryCount(storageutil.GolemDBAddress)
	assert.Greater(t, entriesBefore, 0)

	// Clear the set
	bytekeyset.Clear(db, setKey)

	// Verify the set is empty
	for _, value := range values {
		assert.False(t, bytekeyset.ContainsValue(db, setKey, value))
	}
	size = bytekeyset.Size(db, setKey)
	assert.Equal(t, uint64(0), size.Uint64())

	// Storage should be completely empty after clearing
	assert.Equal(t, 0, db.GetStorageEntryCount(storageutil.GolemDBAddress))
}

func TestClearAndReaddValues(t *testing.T) {
	db := newMockStateAccess()
	setKey := newHash("0x1")
	value1 := byte(2)
	value2 := byte(3)

	// Add values
	err := bytekeyset.AddValue(db, setKey, value1)
	assert.NoError(t, err)
	err = bytekeyset.AddValue(db, setKey, value2)
	assert.NoError(t, err)

	// Storage should have entries
	entriesBefore := db.GetStorageEntryCount(storageutil.GolemDBAddress)
	assert.Greater(t, entriesBefore, 0)

	// Clear the set
	bytekeyset.Clear(db, setKey)

	// Verify the set is empty
	size := bytekeyset.Size(db, setKey)
	assert.Equal(t, uint64(0), size.Uint64())

	// Storage should be empty after clearing
	assert.Equal(t, 0, db.GetStorageEntryCount(storageutil.GolemDBAddress))

	// Add values again
	err = bytekeyset.AddValue(db, setKey, value1)
	assert.NoError(t, err)
	err = bytekeyset.AddValue(db, setKey, value2)
	assert.NoError(t, err)

	// Verify the values were added correctly
	assert.True(t, bytekeyset.ContainsValue(db, setKey, value1))
	assert.True(t, bytekeyset.ContainsValue(db, setKey, value2))
	size = bytekeyset.Size(db, setKey)
	assert.Equal(t, uint64(2), size.Uint64())

	// Storage should have entries again
	entriesAfter := db.GetStorageEntryCount(storageutil.GolemDBAddress)
	assert.Greater(t, entriesAfter, 0)
}

func TestIterateEmptySet(t *testing.T) {
	db := newMockStateAccess()
	setKey := newHash("0x1")

	assert.Empty(t, slices.Collect(bytekeyset.Iterate(db, setKey)))
}

func TestIterateSetWithSingleValue(t *testing.T) {
	db := newMockStateAccess()
	setKey := newHash("0x1")
	value := byte(2)

	// Add a value
	err := bytekeyset.AddValue(db, setKey, value)
	assert.NoError(t, err)

	values := slices.Collect(bytekeyset.Iterate(db, setKey))

	// Should find exactly one value
	assert.Equal(t, 1, len(values))
	assert.Equal(t, value, values[0])
}

func TestIterateSetWithMultipleValues(t *testing.T) {
	db := newMockStateAccess()
	setKey := newHash("0x1")
	value1 := byte(2)
	value2 := byte(3)
	value3 := byte(4)

	// Add multiple values
	err := bytekeyset.AddValue(db, setKey, value1)
	assert.NoError(t, err)

	err = bytekeyset.AddValue(db, setKey, value2)
	assert.NoError(t, err)

	err = bytekeyset.AddValue(db, setKey, value3)
	assert.NoError(t, err)

	// Verify all values are in the set using Size
	assert.Equal(t, uint64(3), bytekeyset.Size(db, setKey).Uint64())

	values := slices.Collect(bytekeyset.Iterate(db, setKey))

	// Should find all three values
	assert.Equal(t, 3, len(values))
	assert.Contains(t, values, value1)
	assert.Contains(t, values, value2)
	assert.Contains(t, values, value3)
}

func TestIterateWithEarlyTermination(t *testing.T) {
	db := newMockStateAccess()
	setKey := newHash("0x1")
	value1 := byte(2)
	value2 := byte(3)
	value3 := byte(4)

	// Add multiple values
	err := bytekeyset.AddValue(db, setKey, value1)
	assert.NoError(t, err)

	err = bytekeyset.AddValue(db, setKey, value2)
	assert.NoError(t, err)

	err = bytekeyset.AddValue(db, setKey, value3)
	assert.NoError(t, err)

	iterationCount := 0

	for range bytekeyset.Iterate(db, setKey) {
		iterationCount++
		if iterationCount >= 2 {
			break
		}
	}

	// Should have stopped after the second value
	assert.Equal(t, 2, iterationCount)
}

func TestIterateAfterRemovingMiddleValue(t *testing.T) {
	db := newMockStateAccess()
	setKey := newHash("0x1")
	value1 := byte(41)
	value2 := byte(42)
	value3 := byte(43)

	// Add multiple values
	err := bytekeyset.AddValue(db, setKey, value1)
	assert.NoError(t, err)

	err = bytekeyset.AddValue(db, setKey, value2)
	assert.NoError(t, err)

	err = bytekeyset.AddValue(db, setKey, value3)
	assert.NoError(t, err)

	// Remove the middle value
	err = bytekeyset.RemoveValue(db, setKey, value2)
	assert.NoError(t, err)

	valuesAfterRemoval := slices.Collect(bytekeyset.Iterate(db, setKey))

	// Should have two values - value1 and value3
	assert.Equal(t, 2, len(valuesAfterRemoval))
	assert.Contains(t, valuesAfterRemoval, value1)
	assert.Contains(t, valuesAfterRemoval, value3)
	assert.NotContains(t, valuesAfterRemoval, value2)
}
