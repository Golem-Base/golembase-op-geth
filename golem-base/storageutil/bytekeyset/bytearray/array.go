package bytearray

import (
	"errors"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/golem-base/storageutil"
	"github.com/holiman/uint256"
)

var uintConst32 = uint256.NewInt(32)

type ByteArray struct {
	db      storageutil.StateAccess
	address common.Hash
}

func NewArray(db storageutil.StateAccess, address common.Hash) *ByteArray {
	return &ByteArray{db: db, address: address}
}

func (a *ByteArray) Size() *uint256.Int {
	return new(uint256.Int).SetBytes32(a.db.GetState(storageutil.GolemDBAddress, a.address).Bytes())
}

var ErrIndexOutOfBounds = errors.New("index out of bounds")

func (a *ByteArray) Get(index *uint256.Int) (byte, error) {
	size := a.Size()
	if index.Cmp(size) >= 0 {
		return 0, ErrIndexOutOfBounds
	}

	slotIx := uint256.NewInt(0)
	offsetUint := uint256.NewInt(0)
	slotIx, offsetUint = slotIx.DivMod(index, uintConst32, offsetUint)
	offset := offsetUint.Uint64()

	valueAddress := uint256.NewInt(0).SetBytes32(a.address.Bytes())
	// The first slot holds the array size
	valueAddress.AddUint64(valueAddress, 1)
	valueAddress.Add(valueAddress, slotIx)

	bytes := a.db.GetState(storageutil.GolemDBAddress, common.Hash(valueAddress.Bytes32())).Bytes()

	return bytes[offset], nil
}

func (a *ByteArray) Append(value byte) {
	// Get the new size
	size := a.Size()
	size = size.AddUint64(size, 1)

	// Calculate where the new byte will go
	slotIx := uint256.NewInt(0).Set(size)
	slotIx = slotIx.SubUint64(slotIx, 1)
	offsetUint := uint256.NewInt(0)
	slotIx, offsetUint = slotIx.DivMod(slotIx, uintConst32, offsetUint)
	offset := offsetUint.Uint64()

	newElementAddress := uint256.NewInt(0).SetBytes32(a.address.Bytes())
	// The first slot holds the array size
	newElementAddress.AddUint64(newElementAddress, 1)
	newElementAddress.Add(newElementAddress, slotIx)

	val := a.db.GetState(storageutil.GolemDBAddress, common.Hash(newElementAddress.Bytes32())).Bytes()
	val[offset] = value
	a.db.SetState(storageutil.GolemDBAddress, common.Hash(newElementAddress.Bytes32()), common.BytesToHash(val))

	a.db.SetState(storageutil.GolemDBAddress, a.address, size.Bytes32())
}

var ErrArrayEmpty = errors.New("array is empty")

func (a *ByteArray) RemoveLast() error {
	size := a.Size()
	if size.CmpUint64(0) == 0 {
		return ErrArrayEmpty
	}

	size.SubUint64(size, 1)
	a.db.SetState(storageutil.GolemDBAddress, a.address, size.Bytes32())

	slotIx := uint256.NewInt(0)
	offset := uint256.NewInt(0)
	slotIx, offset = slotIx.DivMod(size, uintConst32, offset)

	// If the whole 32 byte slot became unused, we wipe it
	if offset.CmpUint64(0) == 0 {
		valueAddress := new(uint256.Int).SetBytes32(a.address.Bytes())
		// The first slot holds the array size
		valueAddress.AddUint64(valueAddress, 1)
		valueAddress.Add(valueAddress, slotIx)
		a.db.SetState(storageutil.GolemDBAddress, common.Hash(valueAddress.Bytes32()), common.Hash{})
	}

	return nil
}

func (a *ByteArray) Set(index *uint256.Int, value byte) error {
	size := a.Size()
	if index.Cmp(size) >= 0 {
		return ErrIndexOutOfBounds
	}

	slotIx := uint256.NewInt(0)
	offsetUint := uint256.NewInt(0)
	slotIx, offsetUint = slotIx.DivMod(index, uintConst32, offsetUint)
	offset := offsetUint.Uint64()

	valueAddress := uint256.NewInt(0).SetBytes32(a.address.Bytes())
	// The first slot holds the array size
	valueAddress.AddUint64(valueAddress, 1)
	valueAddress.Add(valueAddress, slotIx)

	val := a.db.GetState(storageutil.GolemDBAddress, common.Hash(valueAddress.Bytes32())).Bytes()
	val[offset] = value
	a.db.SetState(storageutil.GolemDBAddress, common.Hash(valueAddress.Bytes32()), common.BytesToHash(val))

	return nil
}

func (a *ByteArray) Iterate(yield func(value byte) bool) {
	size := a.Size()
	ix := new(uint256.Int).SetUint64(0)

	cutoff, rest := uint256.NewInt(0), uint256.NewInt(0)
	cutoff, rest = cutoff.DivMod(size, uintConst32, rest)

	for ; ix.Cmp(cutoff) <= 0; ix.AddUint64(ix, 1) {
		valueAddress := uint256.NewInt(0).SetBytes32(a.address.Bytes())
		// The first slot holds the array size
		valueAddress.AddUint64(valueAddress, 1)
		valueAddress.Add(valueAddress, ix)

		val := a.db.GetState(storageutil.GolemDBAddress, common.Hash(valueAddress.Bytes32())).Bytes()

		if ix.Cmp(cutoff) < 0 {
			for _, b := range val {
				if !yield(b) {
					return
				}
			}
		} else {
			for i := range rest.Uint64() {
				if !yield(val[i]) {
					return
				}
			}
		}
	}
}

func (a *ByteArray) Clear() {
	size := a.Size()
	slotIx := uint256.NewInt(0).Set(size)
	offsetUint := uint256.NewInt(0)
	slotIx, offsetUint = slotIx.DivMod(slotIx, uintConst32, offsetUint)
	offset := offsetUint.Uint64()

	lastAddress := uint256.NewInt(0).SetBytes32(a.address.Bytes())
	// The first slot holds the array size
	lastAddress.AddUint64(lastAddress, 1)
	lastAddress.Add(lastAddress, slotIx)
	if offset > 0 {
		// We have an additional slot that's not completely full
		lastAddress.AddUint64(lastAddress, 1)
	}

	for address := new(uint256.Int).SetBytes32(a.address.Bytes()); address.Cmp(lastAddress) < 0; address.AddUint64(address, 1) {
		a.db.SetState(storageutil.GolemDBAddress, common.Hash(address.Bytes32()), common.Hash{})
	}
}
