package stringannotationindex

import (
	"fmt"
	"slices"
	"sort"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/golem-base/storageutil"
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

func TestAddAnnotation(t *testing.T) {
	db := newMockStateAccess()
	ix := NewIndex(db, "TestAddAnnotation")

	require.NoError(t, ix.addEntity("test",
		common.HexToHash("0xe1"),
		common.HexToHash("0xe2"),
		common.HexToHash("0xe3"),
		common.HexToHash("0xe4"),
		common.HexToHash("0xe5"),
		common.HexToHash("0xe6"),
	))
	require.NoError(t, ix.addEntity("tester",
		common.HexToHash("0xe1"),
		common.HexToHash("0xe2"),
		common.HexToHash("0xe3"),
		common.HexToHash("0xe4"),
		common.HexToHash("0xe5"),
		common.HexToHash("0xe6"),
	))
	require.NoError(t, ix.addEntity("testers",
		common.HexToHash("0xe1"),
		common.HexToHash("0xe2"),
		common.HexToHash("0xe3"),
		common.HexToHash("0xe4"),
		common.HexToHash("0xe5"),
		common.HexToHash("0xe6"),
	))
	require.NoError(t, ix.addEntity("foo",
		common.HexToHash("0xe1"),
		common.HexToHash("0xe2"),
		common.HexToHash("0xe3"),
		common.HexToHash("0xe4"),
		common.HexToHash("0xe5"),
		common.HexToHash("0xe6"),
	))
	require.NoError(t, ix.addEntity("foobar",
		common.HexToHash("0xe1"),
		common.HexToHash("0xe2"),
		common.HexToHash("0xe3"),
	))
	require.NoError(t, ix.addEntity("fooqar",
		common.HexToHash("0xe1"),
		common.HexToHash("0xe2"),
		common.HexToHash("0xe3"),
	))
	require.NoError(t, ix.addEntity("foozar",
		common.HexToHash("0xe1"),
		common.HexToHash("0xe2"),
		common.HexToHash("0xe3"),
	))
	require.Equal(t, 601, db.GetStorageEntryCount(storageutil.GolemDBAddress))
}

func TestContaining(t *testing.T) {
	db := newMockStateAccess()
	ix := NewIndex(db, "TestContaining")

	require.NoError(t, ix.addEntity("xy",
		common.HexToHash("0xe1"),
		common.HexToHash("0xe2"),
	))
	require.ElementsMatch(
		t,
		slices.Collect(ix.findEntitiesContaining("x")),
		[]common.Hash{
			common.HexToHash("0xe1"),
			common.HexToHash("0xe2"),
		},
	)

	require.NoError(t, ix.addEntity("test",
		common.HexToHash("0xe1"),
		common.HexToHash("0xe2"),
		common.HexToHash("0xe3"),
	))
	require.NoError(t, ix.addEntity("fossball",
		common.HexToHash("0xe4"),
	))
	require.NoError(t, ix.addEntity("foobar",
		common.HexToHash("0xe5"),
	))
	require.NoError(t, ix.addEntity("quxbar",
		common.HexToHash("0xe6"),
	))
	require.NoError(t, ix.addEntity("bar",
		common.HexToHash("0xe7"),
	))

	require.ElementsMatch(
		t,
		slices.Collect(ix.findEntitiesContaining("s")),
		[]common.Hash{
			common.HexToHash("0xe1"),
			common.HexToHash("0xe2"),
			common.HexToHash("0xe3"),
			common.HexToHash("0xe4"),
		},
	)

	require.ElementsMatch(
		t,
		slices.Collect(ix.findEntitiesContaining("es")),
		[]common.Hash{
			common.HexToHash("0xe1"),
			common.HexToHash("0xe2"),
			common.HexToHash("0xe3"),
		},
	)

	require.ElementsMatch(
		t,
		slices.Collect(ix.findEntitiesContaining("fo")),
		[]common.Hash{
			common.HexToHash("0xe4"),
			common.HexToHash("0xe5"),
		},
	)

	require.ElementsMatch(
		t,
		slices.Collect(ix.findEntitiesContaining("bar")),
		[]common.Hash{
			common.HexToHash("0xe5"),
			common.HexToHash("0xe6"),
			common.HexToHash("0xe7"),
		},
	)

	require.ElementsMatch(
		t,
		slices.Collect(ix.findEntitiesEndingWith("bar")),
		[]common.Hash{
			common.HexToHash("0xe5"),
			common.HexToHash("0xe6"),
			common.HexToHash("0xe7"),
		},
	)

	require.ElementsMatch(
		t,
		slices.Collect(ix.findEntitiesEndingWith("test")),
		[]common.Hash{
			common.HexToHash("0xe1"),
			common.HexToHash("0xe2"),
			common.HexToHash("0xe3"),
		},
	)

	require.ElementsMatch(
		t,
		slices.Collect(ix.findEntitiesStartingWith("test")),
		[]common.Hash{
			common.HexToHash("0xe1"),
			common.HexToHash("0xe2"),
			common.HexToHash("0xe3"),
		},
	)

	require.ElementsMatch(
		t,
		slices.Collect(ix.findEntitiesStartingWith("foo")),
		[]common.Hash{
			common.HexToHash("0xe5"),
		},
	)

	require.ElementsMatch(
		t,
		slices.Collect(ix.findEntitiesStartingWith("fo")),
		[]common.Hash{
			common.HexToHash("0xe4"),
			common.HexToHash("0xe5"),
		},
	)
}

func TestAddRemoveAnnotation(t *testing.T) {
	db := newMockStateAccess()
	ix := NewIndex(db, "TestAddRemoveAnnotation")

	require.NoError(t, ix.addEntity("t", common.HexToHash("0xe6")))
	require.Equal(t, 7, db.GetStorageEntryCount(storageutil.GolemDBAddress))
	require.NoError(t, ix.addEntity("t", common.HexToHash("0xe5")))
	require.Equal(t, 9, db.GetStorageEntryCount(storageutil.GolemDBAddress))

	require.NoError(t, ix.removeEntity("t", common.HexToHash("0xe6")))
	require.Equal(t, 7, db.GetStorageEntryCount(storageutil.GolemDBAddress))
	require.NoError(t, ix.removeEntity("t", common.HexToHash("0xe5")))
	require.Equal(t, 0, db.GetStorageEntryCount(storageutil.GolemDBAddress))

	require.NoError(t, ix.addEntity("t", common.HexToHash("0xe6")))
	require.Equal(t, 7, db.GetStorageEntryCount(storageutil.GolemDBAddress))

	require.NoError(t, ix.addEntity("t", common.HexToHash("0xe5")))
	require.Equal(t, 9, db.GetStorageEntryCount(storageutil.GolemDBAddress))

	require.NoError(t, ix.addEntity("testme", common.HexToHash("0xe7")))
	require.Equal(t, 95, db.GetStorageEntryCount(storageutil.GolemDBAddress))

	require.NoError(t, ix.removeEntity("t", common.HexToHash("0xe6")))
	require.Equal(t, 93, db.GetStorageEntryCount(storageutil.GolemDBAddress))
	require.NoError(t, ix.removeEntity("t", common.HexToHash("0xe5")))
	require.Equal(t, 90, db.GetStorageEntryCount(storageutil.GolemDBAddress))
	require.NoError(t, ix.removeEntity("testme", common.HexToHash("0xe7")))

	require.Equal(t, 0, db.GetStorageEntryCount(storageutil.GolemDBAddress))
}
