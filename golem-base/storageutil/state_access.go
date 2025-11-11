package storageutil

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/holiman/uint256"
)

type StateAccess interface {
	GetState(common.Address, common.Hash) common.Hash
	SetState(common.Address, common.Hash, common.Hash) common.Hash
	AddBalance(addr common.Address, amount *uint256.Int, reason tracing.BalanceChangeReason) uint256.Int
	SubBalance(addr common.Address, amount *uint256.Int, reason tracing.BalanceChangeReason) uint256.Int
}
