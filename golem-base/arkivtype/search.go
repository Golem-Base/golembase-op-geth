package arkivtype

import (
	"encoding/json"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity"
)

type OrderBy struct {
	Column     string `json:"column"`
	Descending bool   `json:"descending"`
}

type QueryOptions struct {
	AtBlock            *uint64     `json:"at_block"`
	IncludeAnnotations bool        `json:"include_annotations"`
	Columns            []string    `json:"columns"`
	From               []FromEntry `json:"from"`
	OrderBy            []OrderBy   `json:"order_by"`
}

type FromEntry struct {
	Column string `json:"column"`
	Value  any    `json:"value"`
}

type NextData struct {
	AtBlock uint64      `json:"at_block"`
	From    []FromEntry `json:"query_from"`
}

func (r QueryEntitiesResult) HasNext() bool {
	return len(r.Next.From) > 0
}

type QueryEntitiesResult struct {
	From    common.Hash    `json:"from"`
	Results []SearchResult `json:"results"`
	Next    NextData       `json:"next"`
}

type SearchResult struct {
	Key       common.Hash    `json:"key"`
	Value     []byte         `json:"value"`
	ExpiresAt uint64         `json:"expires_at"`
	Owner     common.Address `json:"owner"`

	StringAnnotations  []entity.StringAnnotation  `json:"string_annotations"`
	NumericAnnotations []entity.NumericAnnotation `json:"numeric_annotations"`
}

func (r *SearchResult) EncodedSize() (int, error) {
	encoded, err := json.Marshal(r)
	if err != nil {
		return 0, fmt.Errorf("error encoding value: %w", err)
	}
	return len(encoded), nil
}
