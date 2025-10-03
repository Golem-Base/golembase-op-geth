package arkivtype

import (
	"github.com/ethereum/go-ethereum/common"
)

type SearchResult struct {
	Key       common.Hash    `json:"key"`
	Value     []byte         `json:"value"`
	ExpiresAt uint64         `json:"expires_at"`
	Owner     common.Address `json:"owner"`
}
