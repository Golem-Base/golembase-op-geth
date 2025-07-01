package query_test

import (
	"fmt"
	"slices"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/golem-base/query"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/numericalannotationindex"
	"github.com/stretchr/testify/require"
)

// var _ query.Evaluator = &query.EqualExpr{}

var rngSeed common.Hash = common.HexToHash("0x123456789")

func AddAndCheck(t *testing.T, ix numericalannotationindex.Index, value uint64, entityKeys ...common.Hash) {
	err := ix.Add(value, entityKeys...)

	require.NoError(t, err, "adding returned an error")
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

type fakeDataSource struct {
	stringAnnotations  map[string]map[string][]common.Hash
	numericAnnotations map[string]numericalannotationindex.Index
	ownerAddresses     map[common.Address][]common.Hash
}

func (f *fakeDataSource) GetKeysForStringAnnotation(key, value string) ([]common.Hash, error) {
	return f.stringAnnotations[key][value], nil
}

func (f *fakeDataSource) GetKeysForNumericAnnotation(key string, value uint64) ([]common.Hash, error) {
	return slices.Collect(f.numericAnnotations[key].IterateFromTo(&value, &value)), nil
}

func (f *fakeDataSource) GetKeysForOwner(owner common.Address) ([]common.Hash, error) {
	return f.ownerAddresses[owner], nil
}

func (f *fakeDataSource) GetKeysForNumericAnnotationRange(key string, from *uint64, to *uint64) ([]common.Hash, error) {
	return slices.Collect(f.numericAnnotations[key].IterateFromTo(from, to)), nil
}

func TestEqualExpr(t *testing.T) {
	ds := &fakeDataSource{
		stringAnnotations: map[string]map[string][]common.Hash{
			"name": {
				"test":  []common.Hash{common.HexToHash("0x1")},
				"test2": []common.Hash{common.HexToHash("0x2")},
			},
			"déçevant": {
				"non": []common.Hash{common.HexToHash("0x3")},
			},
			"بروح": {
				"ايوة": []common.Hash{common.HexToHash("0x3")},
			},
		},
		numericAnnotations: map[string]numericalannotationindex.Index{},
	}

	expr, err := query.Parse("name = \"test\"")
	require.NoError(t, err)

	res, err := expr.Evaluate(ds)
	require.NoError(t, err)

	require.Equal(t, []common.Hash{common.HexToHash("0x1")}, res)

	// Query for a key with special characters
	expr, err = query.Parse("déçevant = \"non\"")
	require.NoError(t, err)

	res, err = expr.Evaluate(ds)
	require.NoError(t, err)

	require.Equal(t, []common.Hash{common.HexToHash("0x3")}, res)

	expr, err = query.Parse("بروح = \"ايوة\"")
	require.NoError(t, err)

	res, err = expr.Evaluate(ds)
	require.NoError(t, err)

	require.Equal(t, []common.Hash{common.HexToHash("0x3")}, res)

	// But symbols should fail
	_, err = query.Parse("foo@ = \"bar\"")
	require.Error(t, err)
}

func TestNumericEqualExpr(t *testing.T) {
	state := newMockStateAccess()
	ageIx := numericalannotationindex.New(state, "age", rngSeed)
	ds := &fakeDataSource{
		stringAnnotations: map[string]map[string][]common.Hash{},
		numericAnnotations: map[string]numericalannotationindex.Index{
			"age": ageIx,
		},
	}
	AddAndCheck(t, ageIx, 123, common.HexToHash("0x1"))
	AddAndCheck(t, ageIx, 456, common.HexToHash("0x2"))

	expr, err := query.Parse("age = 123")
	require.NoError(t, err)

	res, err := expr.Evaluate(ds)
	require.NoError(t, err)
	require.Equal(t, []common.Hash{common.HexToHash("0x1")}, res)
}

func TestNumericRangeExpr(t *testing.T) {
	state := newMockStateAccess()
	ageIx := numericalannotationindex.New(state, "age", rngSeed)
	ds := &fakeDataSource{
		stringAnnotations: map[string]map[string][]common.Hash{},
		numericAnnotations: map[string]numericalannotationindex.Index{
			"age": ageIx,
		},
	}
	AddAndCheck(t, ageIx, 123, common.HexToHash("0x1"))
	AddAndCheck(t, ageIx, 124, common.HexToHash("0x124"))
	AddAndCheck(t, ageIx, 456, common.HexToHash("0x2"))

	expr, err := query.Parse("age < 124")
	require.NoError(t, err)

	res, err := expr.Evaluate(ds)
	require.NoError(t, err)
	require.Equal(t, []common.Hash{common.HexToHash("0x1")}, res)

	expr, err = query.Parse("age <= 124")
	require.NoError(t, err)

	res, err = expr.Evaluate(ds)
	require.NoError(t, err)
	require.Equal(t, []common.Hash{common.HexToHash("0x1"), common.HexToHash("0x124")}, res)

	expr, err = query.Parse("age > 124")
	require.NoError(t, err)

	res, err = expr.Evaluate(ds)
	require.NoError(t, err)
	require.Equal(t, []common.Hash{common.HexToHash("0x2")}, res)

	expr, err = query.Parse("age >= 124")
	require.NoError(t, err)

	res, err = expr.Evaluate(ds)
	require.NoError(t, err)
	require.Equal(t, []common.Hash{common.HexToHash("0x124"), common.HexToHash("0x2")}, res)
}

func TestAndExpr(t *testing.T) {
	state := newMockStateAccess()
	ageIx := numericalannotationindex.New(state, "age", rngSeed)
	ds := &fakeDataSource{
		stringAnnotations: map[string]map[string][]common.Hash{
			"name": {
				"abc": []common.Hash{common.HexToHash("0x1"), common.HexToHash("0x3")},
			},
		},
		numericAnnotations: map[string]numericalannotationindex.Index{
			"age": ageIx,
		},
	}
	AddAndCheck(t, ageIx, 123, common.HexToHash("0x1"), common.HexToHash("0x2"))

	expr, err := query.Parse(`age = 123 && name = "abc"`)
	require.NoError(t, err)

	res, err := expr.Evaluate(ds)
	require.NoError(t, err)
	require.Equal(t, []common.Hash{common.HexToHash("0x1")}, res)
}

func TestOrExpr(t *testing.T) {
	state := newMockStateAccess()
	ageIx := numericalannotationindex.New(state, "age", rngSeed)
	ds := &fakeDataSource{
		stringAnnotations: map[string]map[string][]common.Hash{
			"name": {
				"abc": []common.Hash{common.HexToHash("0x3")},
			},
		},
		numericAnnotations: map[string]numericalannotationindex.Index{
			"age": ageIx,
		},
	}
	AddAndCheck(t, ageIx, 123, common.HexToHash("0x1"), common.HexToHash("0x2"))

	expr, err := query.Parse(`age = 123 || name = "abc"`)
	require.NoError(t, err)

	res, err := expr.Evaluate(ds)
	require.NoError(t, err)
	require.ElementsMatch(t, []common.Hash{
		common.HexToHash("0x1"),
		common.HexToHash("0x2"),
		common.HexToHash("0x3"),
	}, res)
}

func TestParenthesesExpr(t *testing.T) {
	state := newMockStateAccess()
	nameIx := numericalannotationindex.New(state, "name", rngSeed)
	name4Ix := numericalannotationindex.New(state, "name4", rngSeed)
	ds := &fakeDataSource{
		stringAnnotations: map[string]map[string][]common.Hash{
			"name2": {
				"abc": []common.Hash{common.HexToHash("0x2"), common.HexToHash("0x3")},
			},
			"name3": {
				"def": []common.Hash{common.HexToHash("0x3"), common.HexToHash("0x4")},
			},
		},
		numericAnnotations: map[string]numericalannotationindex.Index{
			"name":  nameIx,
			"name4": name4Ix,
		},
	}

	AddAndCheck(t, nameIx, 123, common.HexToHash("0x1"), common.HexToHash("0x2"))
	AddAndCheck(t, name4Ix, 456, common.HexToHash("0x5"))

	expr, err := query.Parse(`(name = 123 || name2 = "abc") && name3 = "def" || name4 = 456`)
	require.NoError(t, err)

	res, err := expr.Evaluate(ds)
	require.NoError(t, err)
	require.ElementsMatch(t, []common.Hash{
		common.HexToHash("0x3"),
		common.HexToHash("0x5"),
	}, res)
}

func TestOwner(t *testing.T) {
	owner := common.HexToAddress("0x1")

	state := newMockStateAccess()
	ageIx := numericalannotationindex.New(state, "age", rngSeed)
	ds := &fakeDataSource{
		stringAnnotations: map[string]map[string][]common.Hash{
			"name": {
				"abc": []common.Hash{common.HexToHash("0x3")},
			},
		},
		numericAnnotations: map[string]numericalannotationindex.Index{
			"age": ageIx,
		},
		ownerAddresses: map[common.Address][]common.Hash{
			owner: {common.HexToHash("0x1"), common.HexToHash("0x3")},
		},
	}

	AddAndCheck(t, ageIx, 123, common.HexToHash("0x1"), common.HexToHash("0x2"))

	expr, err := query.Parse(fmt.Sprintf(`(age = 123 || name = "abc") && $owner = "%s"`, owner))
	require.NoError(t, err)

	res, err := expr.Evaluate(ds)
	require.NoError(t, err)
	require.ElementsMatch(t, []common.Hash{
		common.HexToHash("0x1"),
		common.HexToHash("0x3"),
	}, res)
}
