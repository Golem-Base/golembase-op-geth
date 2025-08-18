package bytekeyset

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/golem-base/storageutil"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/bytekeyset/bytearray"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/keyset"
	"github.com/holiman/uint256"
)

type StateAccess = storageutil.StateAccess

type Impl struct{}

func (Impl) NewArray(db storageutil.StateAccess, address common.Hash) keyset.Array[byte] {
	return bytearray.NewArray(db, address)
}

func (Impl) ValueToHash(value byte) common.Hash {
	return common.BytesToHash([]byte{value})
}

var impl keyset.StorageImpl[byte] = Impl{}

func ContainsValue(db StateAccess, setKey common.Hash, value byte) bool {
	return keyset.GenericContainsValue(db, impl, setKey, value)
}

func AddValue(db StateAccess, setKey common.Hash, value byte) error {
	return keyset.GenericAddValue(db, impl, setKey, value)
}

func RemoveValue(db StateAccess, setKey common.Hash, value byte) error {
	return keyset.GenericRemoveValue(db, impl, setKey, value)
}

func Size(db StateAccess, setKey common.Hash) *uint256.Int {
	return keyset.GenericSize(db, impl, setKey)
}

func Clear(db StateAccess, setKey common.Hash) {
	keyset.GenericClear(db, impl, setKey)
}

func Iterate(db StateAccess, setKey common.Hash) func(yield func(value byte) bool) {
	return keyset.GenericIterate(db, impl, setKey)
}
