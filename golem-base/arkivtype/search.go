package arkivtype

import (
	"encoding/json"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity"
)

type QueryResponse struct {
	Data        []json.RawMessage `json:"data"`
	BlockNumber uint64            `json:"blockNumber"`
	Cursor      uint64            `json:"cursor,omitempty,string"`
}

type EntityData struct {
	Key       common.Hash    `json:"key,omitempty"`
	Value     hexutil.Bytes  `json:"value,omitempty"`
	ExpiresAt uint64         `json:"expiresAt,omitempty"`
	Owner     common.Address `json:"owner,omitempty"`

	StringAnnotations  []entity.StringAnnotation  `json:"stringAnnotations,omitempty"`
	NumericAnnotations []entity.NumericAnnotation `json:"numericAnnotations,omitempty"`
}
