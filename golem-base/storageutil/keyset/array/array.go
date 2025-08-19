package array

import (
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/golem-base/storageutil"
	"github.com/holiman/uint256"
)

type Array struct {
	db      storageutil.StateAccess
	address common.Hash
}

func NewArray(db storageutil.StateAccess, address common.Hash) *Array {
	return &Array{db: db, address: address}
}

func (a *Array) Size() *uint256.Int {
	return new(uint256.Int).SetBytes32(a.db.GetState(storageutil.GolemDBAddress, a.address).Bytes())
}

var ErrIndexOutOfBounds = errors.New("index out of bounds")

func (a *Array) Get(index *uint256.Int) (common.Hash, error) {
	size := a.Size()
	if index.Cmp(size) >= 0 {
		return common.Hash{}, ErrIndexOutOfBounds
	}

	startAddress := new(uint256.Int).SetBytes32(a.address.Bytes())
	startAddress.Add(startAddress, index)
	startAddress.AddUint64(startAddress, 1)

	return a.db.GetState(storageutil.GolemDBAddress, common.Hash(startAddress.Bytes32())), nil
}

func (a *Array) Append(value common.Hash) {
	size := a.Size()

	newElementAddress := new(uint256.Int).SetBytes32(a.address.Bytes())
	newElementAddress.Add(newElementAddress, size)
	newElementAddress.AddUint64(newElementAddress, 1)

	a.db.SetState(storageutil.GolemDBAddress, common.Hash(newElementAddress.Bytes32()), value)

	size.AddUint64(size, 1)
	a.db.SetState(storageutil.GolemDBAddress, a.address, size.Bytes32())
}

var ErrArrayEmpty = errors.New("array is empty")

func (a *Array) RemoveLast() error {
	size := a.Size()
	if size.CmpUint64(0) == 0 {
		return ErrArrayEmpty
	}

	size.SubUint64(size, 1)
	a.db.SetState(storageutil.GolemDBAddress, a.address, size.Bytes32())

	valueAddress := new(uint256.Int).SetBytes32(a.address.Bytes())
	valueAddress.Add(valueAddress, size)
	valueAddress.AddUint64(valueAddress, 1)
	a.db.SetState(storageutil.GolemDBAddress, common.Hash(valueAddress.Bytes32()), common.Hash{})

	return nil
}

func (a *Array) RemoveUnordered(index *uint256.Int) (common.Hash, error) {

	size := a.Size()
	if size.CmpUint64(0) == 0 {
		return common.Hash{}, ErrArrayEmpty
	}

	lastElementIndex := new(uint256.Int).Set(size)
	lastElementIndex.SubUint64(lastElementIndex, 1)
	lastElementValue, err := a.Get(lastElementIndex)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to get last element: %w", err)
	}

	if lastElementIndex.Cmp(index) != 0 {
		a.Set(index, lastElementValue)
	}

	err = a.RemoveLast()
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to remove last element: %w", err)
	}
	return lastElementValue, nil
}

func (a *Array) Set(index *uint256.Int, value common.Hash) error {
	size := a.Size()
	if index.Cmp(size) >= 0 {
		return ErrIndexOutOfBounds
	}

	address := new(uint256.Int).SetBytes32(a.address.Bytes())
	address.Add(address, index)
	address.AddUint64(address, 1)

	a.db.SetState(storageutil.GolemDBAddress, common.Hash(address.Bytes32()), value)

	return nil
}

func (a *Array) Iterate(yield func(value common.Hash) bool) {
	size := a.Size()
	for i := new(uint256.Int).SetUint64(0); i.Cmp(size) < 0; i.AddUint64(i, 1) {
		value, err := a.Get(i)
		if err != nil {
			return
		}
		if !yield(value) {
			return
		}
	}
}

func (a *Array) IterateWithIndex(yield func(index *uint256.Int, value common.Hash) bool) {
	size := a.Size()
	for i := new(uint256.Int).SetUint64(0); i.Cmp(size) < 0; i.AddUint64(i, 1) {
		value, err := a.Get(i)
		if err != nil {
			return
		}
		if !yield(uint256.NewInt(0).SetBytes(i.Bytes()), value) {
			return
		}
	}
}

func (a *Array) Clear() {
	size := a.Size()
	lastAddress := new(uint256.Int).SetBytes32(a.address.Bytes())
	lastAddress.Add(lastAddress, size)
	lastAddress.AddUint64(lastAddress, 1)

	for address := new(uint256.Int).SetBytes32(a.address.Bytes()); address.Cmp(lastAddress) < 0; address.AddUint64(address, 1) {
		a.db.SetState(storageutil.GolemDBAddress, common.Hash(address.Bytes32()), common.Hash{})
	}

}
