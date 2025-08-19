package bytearray_test

import (
	"fmt"
	"slices"
	"sort"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/golem-base/storageutil"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/bytekeyset/bytearray"
	"github.com/holiman/uint256"
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

func TestEmptyArray(t *testing.T) {
	db := newMockStateAccess()
	array := bytearray.NewArray(db, common.HexToHash("0xabc"))

	size := array.Size()
	require.Equal(t, uint256.NewInt(0), size)
}

func TestAppendToEmptyArray(t *testing.T) {
	db := newMockStateAccess()
	array := bytearray.NewArray(db, common.HexToHash("0xabc"))
	require.Equal(t, uint256.NewInt(0), array.Size())

	v := byte(40)

	array.Append(v)

	got, err := array.Get(uint256.NewInt(0))
	require.NoError(t, err)
	require.Equal(t, v, got)

	require.Equal(t, uint256.NewInt(1), array.Size())
}

func TestAppendToNonEmptyArray(t *testing.T) {
	db := newMockStateAccess()
	array := bytearray.NewArray(db, common.HexToHash("0xabc"))
	array.Append(byte(40))
	array.Append(byte(41))

	got, err := array.Get(uint256.NewInt(0))
	require.NoError(t, err)
	require.Equal(t, byte(40), got)

	got, err = array.Get(uint256.NewInt(1))
	require.NoError(t, err)
	require.Equal(t, byte(41), got)

	require.Equal(t, uint256.NewInt(2), array.Size())
}

func TestRemoveLastFromNonEmptyArray(t *testing.T) {
	db := newMockStateAccess()
	array := bytearray.NewArray(db, common.HexToHash("0xabc"))
	array.Append(byte(40))
	array.Append(byte(41))

	err := array.RemoveLast()
	require.NoError(t, err)

	got, err := array.Get(uint256.NewInt(0))
	require.NoError(t, err)
	require.Equal(t, byte(40), got)

	require.Equal(t, uint256.NewInt(1), array.Size())
}

func TestRemoveLastFromArrayWithOneElement(t *testing.T) {
	db := newMockStateAccess()
	array := bytearray.NewArray(db, common.HexToHash("0xabc"))
	array.Append(byte(40))

	err := array.RemoveLast()
	require.NoError(t, err)

	require.Equal(t, uint256.NewInt(0), array.Size())
}

func TestRemoveLastFromLongArray(t *testing.T) {
	db := newMockStateAccess()
	array := bytearray.NewArray(db, common.HexToHash("0xabc"))
	for range 500 {
		array.Append(byte(42))
	}

	require.NoError(t, array.RemoveLast())
	require.Equal(t, uint256.NewInt(499), array.Size())

	for range 13 {
		array.Append(byte(42))
	}

	require.NoError(t, array.RemoveLast())
	require.Equal(t, uint256.NewInt(511), array.Size())
}

func TestRemoveUnordered(t *testing.T) {
	db := newMockStateAccess()
	array := bytearray.NewArray(db, common.HexToHash("0xabc"))
	for range 500 {
		array.Append(byte(1))
	}
	array.Append(byte(3))
	array.Set(uint256.NewInt(201), byte(0))

	el, err := array.RemoveUnordered(uint256.NewInt(201))
	require.NoError(t, err)

	require.Equal(t, uint256.NewInt(500), array.Size())
	require.Equal(t, byte(3), el)
}

func TestSetElementForOneElementArray(t *testing.T) {
	db := newMockStateAccess()
	array := bytearray.NewArray(db, common.HexToHash("0xabc"))
	array.Append(byte(40))

	err := array.Set(uint256.NewInt(0), byte(41))
	require.NoError(t, err)

	got, err := array.Get(uint256.NewInt(0))
	require.NoError(t, err)
	require.Equal(t, byte(41), got)
}

func TestSetElementForNonEmptyArray(t *testing.T) {
	db := newMockStateAccess()
	array := bytearray.NewArray(db, common.HexToHash("0xabc"))
	array.Append(byte(40))
	array.Append(byte(41))

	err := array.Set(uint256.NewInt(1), byte(42))
	require.NoError(t, err)

	got, err := array.Get(uint256.NewInt(1))
	require.NoError(t, err)
	require.Equal(t, byte(42), got)

	got, err = array.Get(uint256.NewInt(0))
	require.NoError(t, err)
	require.Equal(t, byte(40), got)
}

func TestIterate(t *testing.T) {
	db := newMockStateAccess()
	array := bytearray.NewArray(db, common.HexToHash("0xabc"))
	array.Append(byte(40))
	array.Append(byte(41))

	values := slices.Collect(array.Iterate)

	require.Equal(t, []byte{byte(40), byte(41)}, values)

	require.Equal(t, array.Size().Uint64(), uint64(2))
	// +1 because the array stores its size
	require.Equal(t, db.GetStorageEntryCount(storageutil.GolemDBAddress), 1+1)

	// Test also with more values than what fits in a single byte
	for range 500 {
		array.Append(byte(42))
	}
	values = slices.Collect(array.Iterate)
	require.Len(t, values, 502)

	require.Equal(t, array.Size().Uint64(), uint64(502))
	// +1 because the array stores its size
	require.Equal(t, db.GetStorageEntryCount(storageutil.GolemDBAddress), 16+1)

	// Test the case where we align exactly on a multiple of 32 bytes
	for range 10 {
		array.Append(byte(42))
	}
	values = slices.Collect(array.Iterate)
	require.Len(t, values, 512)

	require.Equal(t, array.Size().Uint64(), uint64(512))
	// +1 because the array stores its size
	require.Equal(t, db.GetStorageEntryCount(storageutil.GolemDBAddress), 16+1)
}

func TestClear(t *testing.T) {
	db := newMockStateAccess()
	array := bytearray.NewArray(db, common.HexToHash("0xabc"))
	array.Append(byte(40))
	array.Append(byte(41))

	array.Clear()

	require.Equal(t, uint256.NewInt(0), array.Size())

	for range 500 {
		array.Append(byte(42))
	}

	array.Clear()

	require.Equal(t, uint256.NewInt(0), array.Size())

	for range 512 {
		array.Append(byte(42))
	}

	array.Clear()

	require.Equal(t, uint256.NewInt(0), array.Size())
}

func TestEmptyClear(t *testing.T) {
	db := newMockStateAccess()
	array := bytearray.NewArray(db, common.HexToHash("0xabc"))

	array.Clear()

	require.Equal(t, uint256.NewInt(0), array.Size())
}
