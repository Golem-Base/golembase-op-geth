package keyset_test

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/golem-base/storageutil"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/keyset"
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

// Helper function to create test values
func newHash(val string) common.Hash {
	return common.HexToHash(val)
}

func TestAddAndCheckValueInEmptySet(t *testing.T) {
	db := newMockStateAccess()
	setKey := newHash("0x1")
	value := newHash("0x2")

	// Initially should not contain the value
	assert.False(t, keyset.ContainsValue(db, setKey, value))

	// Add value
	err := keyset.AddValue(db, setKey, value)
	assert.NoError(t, err)

	// Should contain the value after adding
	assert.True(t, keyset.ContainsValue(db, setKey, value))
}

func TestAddDuplicateValue(t *testing.T) {
	db := newMockStateAccess()
	setKey := newHash("0x1")
	value := newHash("0x2")

	// Add value first time
	err := keyset.AddValue(db, setKey, value)
	assert.NoError(t, err)

	// Add same value second time
	err = keyset.AddValue(db, setKey, value)
	assert.NoError(t, err)

	// Should still contain the value
	assert.True(t, keyset.ContainsValue(db, setKey, value))
}

func TestRemoveValueFromSet(t *testing.T) {
	db := newMockStateAccess()
	setKey := newHash("0x1")
	value := newHash("0x2")

	// Add and verify value
	err := keyset.AddValue(db, setKey, value)
	assert.NoError(t, err)
	assert.True(t, keyset.ContainsValue(db, setKey, value))

	// Remove value
	err = keyset.RemoveValue(db, setKey, value)
	assert.NoError(t, err)

	// Should not contain the value after removal
	assert.False(t, keyset.ContainsValue(db, setKey, value))
}

func TestRemoveNonExistentValue(t *testing.T) {
	db := newMockStateAccess()
	setKey := newHash("0x1")
	value := newHash("0x2")

	// Try to remove value that was never added
	err := keyset.RemoveValue(db, setKey, value)
	assert.NoError(t, err)
	assert.False(t, keyset.ContainsValue(db, setKey, value))
}

func TestMultipleValuesInSet(t *testing.T) {
	db := newMockStateAccess()
	setKey := newHash("0x1")
	value1 := newHash("0x2")
	value2 := newHash("0x3")
	value3 := newHash("0x4")

	// Add multiple values
	err := keyset.AddValue(db, setKey, value1)
	assert.NoError(t, err)

	require.Equal(t, keyset.Size(db, setKey).Uint64(), uint64(1))

	err = keyset.AddValue(db, setKey, value2)
	assert.NoError(t, err)

	require.Equal(t, keyset.Size(db, setKey).Uint64(), uint64(2))

	err = keyset.AddValue(db, setKey, value3)
	assert.NoError(t, err)

	require.Equal(t, keyset.Size(db, setKey).Uint64(), uint64(3))

	// Verify all values are in the set
	assert.True(t, keyset.ContainsValue(db, setKey, value1))
	assert.True(t, keyset.ContainsValue(db, setKey, value2))
	assert.True(t, keyset.ContainsValue(db, setKey, value3))

	// Remove middle value
	err = keyset.RemoveValue(db, setKey, value2)
	assert.NoError(t, err)

	// Verify state after removal
	assert.True(t, keyset.ContainsValue(db, setKey, value1))
	assert.False(t, keyset.ContainsValue(db, setKey, value2))
	assert.True(t, keyset.ContainsValue(db, setKey, value3))
}

func TestSizeEmptySet(t *testing.T) {
	db := newMockStateAccess()
	setKey := newHash("0x1")

	// Empty set should have size 0
	size := keyset.Size(db, setKey)
	assert.Equal(t, uint64(0), size.Uint64())
}

func TestSizeAfterAddingValues(t *testing.T) {
	db := newMockStateAccess()
	setKey := newHash("0x1")

	// Initially empty
	size := keyset.Size(db, setKey)
	assert.Equal(t, uint64(0), size.Uint64())

	// Add one value
	value1 := newHash("0x2")
	err := keyset.AddValue(db, setKey, value1)
	assert.NoError(t, err)

	// keyset.Size should be 1
	size = keyset.Size(db, setKey)
	assert.Equal(t, uint64(1), size.Uint64())

	// Add another value
	value2 := newHash("0x3")
	err = keyset.AddValue(db, setKey, value2)
	assert.NoError(t, err)

	// keyset.Size should be 2
	size = keyset.Size(db, setKey)
	assert.Equal(t, uint64(2), size.Uint64())
}

func TestSizeAfterRemovingValues(t *testing.T) {
	db := newMockStateAccess()
	setKey := newHash("0x1")

	// Add two values
	value1 := newHash("0x2")
	value2 := newHash("0x3")

	err := keyset.AddValue(db, setKey, value1)
	assert.NoError(t, err)

	err = keyset.AddValue(db, setKey, value2)
	assert.NoError(t, err)

	// keyset.Size should be 2
	size := keyset.Size(db, setKey)
	assert.Equal(t, uint64(2), size.Uint64())

	// Remove one value
	err = keyset.RemoveValue(db, setKey, value1)
	assert.NoError(t, err)

	// keyset.Size should be 1
	size = keyset.Size(db, setKey)
	assert.Equal(t, uint64(1), size.Uint64())

	// Remove another value
	err = keyset.RemoveValue(db, setKey, value2)
	assert.NoError(t, err)

	// keyset.Size should be 0
	size = keyset.Size(db, setKey)
	assert.Equal(t, uint64(0), size.Uint64())
}

func TestSizeWithDuplicateValues(t *testing.T) {
	db := newMockStateAccess()
	setKey := newHash("0x1")
	value := newHash("0x2")

	// Initially empty
	size := keyset.Size(db, setKey)
	assert.Equal(t, uint64(0), size.Uint64())

	// Add value
	err := keyset.AddValue(db, setKey, value)
	assert.NoError(t, err)

	// keyset.Size should be 1
	size = keyset.Size(db, setKey)
	assert.Equal(t, uint64(1), size.Uint64())

	// Add same value again
	err = keyset.AddValue(db, setKey, value)
	assert.NoError(t, err)

	// keyset.Size should still be 1 (no duplicates)
	size = keyset.Size(db, setKey)
	assert.Equal(t, uint64(1), size.Uint64())
}

func TestClearEmptySet(t *testing.T) {
	db := newMockStateAccess()
	setKey := newHash("0x1")

	// Clear an empty set
	keyset.Clear(db, setKey)

	// Size should still be 0
	size := keyset.Size(db, setKey)
	assert.Equal(t, uint64(0), size.Uint64())

	// Storage should be empty
	assert.Equal(t, 0, db.GetStorageEntryCount(storageutil.GolemDBAddress))
}

func TestClearSetWithSingleValue(t *testing.T) {
	db := newMockStateAccess()
	setKey := newHash("0x1")
	value := newHash("0x2")

	// Add a single value
	err := keyset.AddValue(db, setKey, value)
	assert.NoError(t, err)

	// Verify the value was added
	assert.True(t, keyset.ContainsValue(db, setKey, value))
	size := keyset.Size(db, setKey)
	assert.Equal(t, uint64(1), size.Uint64())

	// Storage should have entries
	entriesBefore := db.GetStorageEntryCount(storageutil.GolemDBAddress)
	assert.Greater(t, entriesBefore, 0)

	// Clear the set
	keyset.Clear(db, setKey)

	// Verify the set is empty
	assert.False(t, keyset.ContainsValue(db, setKey, value))
	size = keyset.Size(db, setKey)
	assert.Equal(t, uint64(0), size.Uint64())

	// Storage should be completely empty after clearing
	assert.Equal(t, 0, db.GetStorageEntryCount(storageutil.GolemDBAddress))
}

func TestClearSetWithMultipleValues(t *testing.T) {
	db := newMockStateAccess()
	setKey := newHash("0x1")
	values := []common.Hash{
		newHash("0x2"),
		newHash("0x3"),
		newHash("0x4"),
		newHash("0x5"),
		newHash("0x6"),
	}

	// Add multiple values
	for _, value := range values {
		err := keyset.AddValue(db, setKey, value)
		assert.NoError(t, err)
		assert.True(t, keyset.ContainsValue(db, setKey, value))
	}

	// Verify the set size
	size := keyset.Size(db, setKey)
	assert.Equal(t, uint64(len(values)), size.Uint64())

	// Storage should have entries
	entriesBefore := db.GetStorageEntryCount(storageutil.GolemDBAddress)
	assert.Greater(t, entriesBefore, 0)

	// Clear the set
	keyset.Clear(db, setKey)

	// Verify the set is empty
	for _, value := range values {
		assert.False(t, keyset.ContainsValue(db, setKey, value))
	}
	size = keyset.Size(db, setKey)
	assert.Equal(t, uint64(0), size.Uint64())

	// Storage should be completely empty after clearing
	assert.Equal(t, 0, db.GetStorageEntryCount(storageutil.GolemDBAddress))
}

func TestClearAndReaddValues(t *testing.T) {
	db := newMockStateAccess()
	setKey := newHash("0x1")
	value1 := newHash("0x2")
	value2 := newHash("0x3")

	// Add values
	err := keyset.AddValue(db, setKey, value1)
	assert.NoError(t, err)
	err = keyset.AddValue(db, setKey, value2)
	assert.NoError(t, err)

	// Storage should have entries
	entriesBefore := db.GetStorageEntryCount(storageutil.GolemDBAddress)
	assert.Greater(t, entriesBefore, 0)

	// Clear the set
	keyset.Clear(db, setKey)

	// Verify the set is empty
	size := keyset.Size(db, setKey)
	assert.Equal(t, uint64(0), size.Uint64())

	// Storage should be empty after clearing
	assert.Equal(t, 0, db.GetStorageEntryCount(storageutil.GolemDBAddress))

	// Add values again
	err = keyset.AddValue(db, setKey, value1)
	assert.NoError(t, err)
	err = keyset.AddValue(db, setKey, value2)
	assert.NoError(t, err)

	// Verify the values were added correctly
	assert.True(t, keyset.ContainsValue(db, setKey, value1))
	assert.True(t, keyset.ContainsValue(db, setKey, value2))
	size = keyset.Size(db, setKey)
	assert.Equal(t, uint64(2), size.Uint64())

	// Storage should have entries again
	entriesAfter := db.GetStorageEntryCount(storageutil.GolemDBAddress)
	assert.Greater(t, entriesAfter, 0)
}
