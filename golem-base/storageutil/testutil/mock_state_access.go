package testutil

import (
	"github.com/ethereum/go-ethereum/common"
)

// MockStateAccess implements StateAccess interface for testing
type MockStateAccess struct {
	storage map[common.Address]map[common.Hash]common.Hash
}

func NewMockStateAccess() *MockStateAccess {
	return &MockStateAccess{
		storage: make(map[common.Address]map[common.Hash]common.Hash),
	}
}

func (m *MockStateAccess) GetState(addr common.Address, key common.Hash) common.Hash {
	if _, exists := m.storage[addr]; !exists {
		return common.Hash{}
	}
	if val, exists := m.storage[addr][key]; exists {
		return val
	}
	return common.Hash{}
}

func (m *MockStateAccess) SetState(addr common.Address, key common.Hash, value common.Hash) common.Hash {
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
func (m *MockStateAccess) GetStorageEntryCount(addr common.Address) int {
	if storageMap, exists := m.storage[addr]; exists {
		return len(storageMap)
	}
	return 0
}
