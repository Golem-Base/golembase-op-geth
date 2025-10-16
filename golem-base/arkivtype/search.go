package arkivtype

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity"
)

type QueryResponse struct {
	Data []EntityData `json:"data"`
}

type EntityData struct {
	Key       common.Hash    `json:"key"`
	Value     hexutil.Bytes  `json:"value"`
	ExpiresAt uint64         `json:"expires_at"`
	Owner     common.Address `json:"owner"`

	StringAnnotations  []entity.StringAnnotation  `json:"string_annotations"`
	NumericAnnotations []entity.NumericAnnotation `json:"numeric_annotations"`
}
