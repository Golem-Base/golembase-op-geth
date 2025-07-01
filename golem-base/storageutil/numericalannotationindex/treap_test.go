package numericalannotationindex

import (
	"encoding/binary"
	"fmt"
	"math/rand/v2"
	"slices"
	"sort"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/golem-base/storageutil"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func uint64Pointer(i uint64) *uint64 {
	return &i
}

func generateSeed() common.Hash {
	uint := new(uint256.Int)

	// First seed used to generate the 32 byte array that we use to seed the CheChe8 RNG.
	// This 32 byte array is derived from a uint64.
	rngSeed := uint.AddUint64(uint, rand.Uint64()).Bytes32()
	rng := rand.NewChaCha8(rngSeed)

	// The actual seed that we'll pass to the AnnotationIndex
	seed := make([]byte, 32)
	rng.Read(seed)

	return common.Hash(seed)
}

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

	//fmt.Printf("SetState: %s -> %s\n", key.Hex(), value.Hex())

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
	fmt.Println("State of the DB:")
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

func randomHash() common.Hash {
	b := make([]byte, 32)
	binary.BigEndian.PutUint64(b[24:], rand.Uint64())
	return common.Hash(b)
}

type TestNumericalIndex interface {
	Index

	AddAndCheck(t *testing.T, value uint64, entityKeys ...common.Hash)

	DeleteAndCheck(t *testing.T, value uint64, entityKeys ...common.Hash)
}

func createTestIndex(db storageutil.StateAccess, key string) TestNumericalIndex {
	return create(db, key, rand.NewChaCha8(generateSeed()))
}

func (ix *annotationIndex) AddAndCheck(t *testing.T, value uint64, entityKeys ...common.Hash) {
	err := ix.Add(value, entityKeys...)

	require.NoError(t, err, "adding returned an error")

	require.NoError(t, ix.Check(), "annotationIndex.Check returned error!")
}

func (ix *annotationIndex) DeleteAndCheck(t *testing.T, value uint64, entityKeys ...common.Hash) {
	err := ix.Delete(value, entityKeys...)

	require.NoError(t, err, "adding returned an error")

	require.NoError(t, ix.Check(), "annotationIndex.Check returned error!")
}

func (ix *annotationIndex) Check() error {

	var checkFrom func(node *Node) error
	checkFrom = func(node *Node) error {
		if node == nil {
			return nil
		}

		left := ix.getNodeAt(node.left)
		right := ix.getNodeAt(node.right)

		if (left != nil && (left.priority < node.priority || left.annotationValue > node.annotationValue)) ||
			(right != nil && (right.priority < node.priority || right.annotationValue < node.annotationValue)) {
			return fmt.Errorf("heap property violated!")
		}

		treeSize := node.numberOfEntities()
		if left != nil {
			treeSize += left.TreeSize()
		}
		if right != nil {
			treeSize += right.TreeSize()
		}
		if node.TreeSize() != treeSize {
			return fmt.Errorf("size incorrect, expecting %d but got %d!", treeSize, node.TreeSize())
		}

		err := checkFrom(left)
		if err != nil {
			return fmt.Errorf("error checking left subtree: %w", err)
		}

		err = checkFrom(right)
		if err != nil {
			return fmt.Errorf("error checking right subtree: %w", err)
		}

		return nil
	}

	return checkFrom(ix.GetRootNode())
}

func TestNonExistingAnnotation(t *testing.T) {
	db := newMockStateAccess()
	index := createTestIndex(db, "TestNonExistingAnnotation")

	// Initially should not contain the value
	require.Len(t, slices.Collect(index.IterateFromTo(uint64Pointer(6), uint64Pointer(6))), 0)
}

func TestAddAndRetrieveValue(t *testing.T) {
	db := newMockStateAccess()
	index := createTestIndex(db, "TestAddAndRetrieveValue")

	// Initially should not contain the value
	require.Len(t, slices.Collect(index.IterateFromTo(uint64Pointer(6), uint64Pointer(6))), 0)

	for value := uint64(100); value < 300; value++ {
		count := rand.UintN(50)
		for i := uint(0); i < count; i++ {
			index.AddAndCheck(t, value, randomHash())
		}
	}

	index.AddAndCheck(t, 5, common.HexToHash("0xf5"))
	index.AddAndCheck(t, 5, common.HexToHash("0xf5"))
	index.AddAndCheck(t, 6, common.HexToHash("0xf6"))
	index.AddAndCheck(t, 6, common.HexToHash("0xe6"))
	index.AddAndCheck(t, 20, common.HexToHash("0d"))
	index.AddAndCheck(t, 5, common.HexToHash("0xe5"))
	index.AddAndCheck(t, 5, common.HexToHash("0xd5"))
	index.AddAndCheck(t, 3, common.HexToHash("0xf3"))
	index.AddAndCheck(t, 4, common.HexToHash("0xf4"))
	index.AddAndCheck(t, 40, common.HexToHash("0xf40"))
	index.AddAndCheck(t, 44, common.HexToHash("0xf44"))

	require.Len(t, slices.Collect(index.IterateFromTo(uint64Pointer(5), uint64Pointer(5))), 3)
	require.Len(t, slices.Collect(index.IterateFromTo(uint64Pointer(6), uint64Pointer(6))), 2)

	index.DeleteAndCheck(t, 6, common.HexToHash("0xf6"))
	require.Len(t, slices.Collect(index.IterateFromTo(uint64Pointer(6), uint64Pointer(6))), 1)
	index.DeleteAndCheck(t, 6, common.HexToHash("0xe6"))
	require.Len(t, slices.Collect(index.IterateFromTo(uint64Pointer(6), uint64Pointer(6))), 0)

	require.Len(t, slices.Collect(index.IterateFromTo(uint64Pointer(1), uint64Pointer(1))), 0)
}

func TestDeletion(t *testing.T) {
	db := newMockStateAccess()
	index := createTestIndex(db, "TestDeletion")

	index.AddAndCheck(t, 5, common.HexToHash("0xf5"))
	require.Equal(t, 4, db.GetStorageEntryCount(storageutil.GolemDBAddress))
	index.DeleteAndCheck(t, 5, common.HexToHash("0xf5"))
	require.Equal(t, 0, db.GetStorageEntryCount(storageutil.GolemDBAddress))

	require.Equal(t, uint64(0), index.Size())
	index.AddAndCheck(t, 5, common.HexToHash("0xf5"))
	require.Equal(t, uint64(1), index.Size())
	index.AddAndCheck(t, 5, common.HexToHash("0xe5"))
	require.Equal(t, uint64(2), index.Size())
	index.AddAndCheck(t, 6, common.HexToHash("0xe6"))
	index.AddAndCheck(t, 7, common.HexToHash("0xe7"))
	index.AddAndCheck(t, 9, common.HexToHash("0xe9"))
	index.AddAndCheck(t, 1, common.HexToHash("0xd1"), common.HexToHash("0xe1"), common.HexToHash("0xf1"))
	require.Equal(t, uint64(8), index.Size())

	index.DeleteAndCheck(t, 6, common.HexToHash("0xe6"))
	index.DeleteAndCheck(t, 5, common.HexToHash("0xe5"), common.HexToHash("0xf5"))
	index.DeleteAndCheck(t, 9, common.HexToHash("0xe9"))
	index.DeleteAndCheck(t, 1, common.HexToHash("0xe1"), common.HexToHash("0xf1"))
	index.DeleteAndCheck(t, 1, common.HexToHash("0xd1"))
	index.DeleteAndCheck(t, 7, common.HexToHash("0xe7"))

	require.Equal(t, 0, db.GetStorageEntryCount(storageutil.GolemDBAddress))
}

func TestIteration(t *testing.T) {
	db := newMockStateAccess()
	index := createTestIndex(db, "TestIteration")

	index.AddAndCheck(t, 5, common.HexToHash("0xf5"))
	index.AddAndCheck(t, 3, common.HexToHash("0xe3"))
	index.AddAndCheck(t, 5, common.HexToHash("0xe5"))
	index.AddAndCheck(t, 6, common.HexToHash("0xe6"))
	index.AddAndCheck(t, 7, common.HexToHash("0xe7"))
	index.AddAndCheck(t, 9, common.HexToHash("0xe9"))
	index.AddAndCheck(t, 1, common.HexToHash("0xe1"))

	// Fill the index with other stuff
	for value := uint64(100); value < 300; value++ {
		count := rand.UintN(50)
		for i := uint(0); i < count; i++ {
			index.AddAndCheck(t, value, randomHash())
		}
	}

	items := slices.Collect(index.IterateFromTo(uint64Pointer(4), uint64Pointer(8)))
	require.Len(t, items, 4)
	require.Subset(t, []common.Hash{
		common.HexToHash("0xf5"),
		common.HexToHash("0xe5"),
		common.HexToHash("0xe6"),
		common.HexToHash("0xe7"),
	}, items)

	items = slices.Collect(index.IterateFromTo(nil, uint64Pointer(4)))
	require.Len(t, items, 2)
	require.Subset(t, []common.Hash{
		common.HexToHash("0xe1"),
		common.HexToHash("0xe3"),
	}, items)
}

func TestIterationFrom(t *testing.T) {
	db := newMockStateAccess()
	index := createTestIndex(db, "TestIterationFrom")

	index.AddAndCheck(t, 5000, common.HexToHash("0xf5"))
	index.AddAndCheck(t, 3000, common.HexToHash("0xe3"))
	index.AddAndCheck(t, 5000, common.HexToHash("0xe5"))
	index.AddAndCheck(t, 6000, common.HexToHash("0xe6"))
	index.AddAndCheck(t, 7000, common.HexToHash("0xe7"))
	index.AddAndCheck(t, 9000, common.HexToHash("0xe9"))
	index.AddAndCheck(t, 1000, common.HexToHash("0xe1"))

	items := slices.Collect(index.IterateFromTo(uint64Pointer(4000), nil))
	require.Len(t, items, 5)
	require.Subset(t, []common.Hash{
		common.HexToHash("0xf5"),
		common.HexToHash("0xe5"),
		common.HexToHash("0xe6"),
		common.HexToHash("0xe7"),
		common.HexToHash("0xe9"),
	}, items)
}

func TestIterationFromManyNodes(t *testing.T) {
	db := newMockStateAccess()
	index := createTestIndex(db, "TestIterationFromManyNodes")

	index.AddAndCheck(t, 5000, common.HexToHash("0xf5"))
	index.AddAndCheck(t, 3000, common.HexToHash("0xe3"))
	index.AddAndCheck(t, 5000, common.HexToHash("0xe5"))
	index.AddAndCheck(t, 6000, common.HexToHash("0xe6"))
	index.AddAndCheck(t, 7000, common.HexToHash("0xe7"))
	index.AddAndCheck(t, 9000, common.HexToHash("0xe9"))
	index.AddAndCheck(t, 1000, common.HexToHash("0xe1"))

	// Fill the index with other stuff
	for value := uint64(0); value < 900; value++ {
		count := rand.UintN(50)
		for i := uint(0); i < count; i++ {
			index.AddAndCheck(t, value, randomHash())
		}
	}

	items := slices.Collect(index.IterateFromTo(uint64Pointer(4000), nil))
	require.Len(t, items, 5)
	require.Subset(t, []common.Hash{
		common.HexToHash("0xf5"),
		common.HexToHash("0xe5"),
		common.HexToHash("0xe6"),
		common.HexToHash("0xe7"),
		common.HexToHash("0xe9"),
	}, items)
}
