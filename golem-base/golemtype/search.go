package golemtype

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity"
)

// Selector defines which optional fields should be included in SearchResult
type Selector uint8

const (
	// SelectorValue includes the value field
	SelectorValue Selector = 1 << iota
	// SelectorMetadata includes the metadata field
	SelectorMetadata
	// SelectorAttributes includes the attributes field
	SelectorAttributes
	// SelectorAll includes all optional fields
	SelectorAll = SelectorValue | SelectorMetadata | SelectorAttributes
)

// SearchResult represents a search result with optional fields
type SearchResult struct {
	Key        common.Hash            `json:"key"`
	Value      *[]byte                `json:"value,omitempty"`
	Metadata   *entity.EntityMetaData `json:"metadata,omitempty"`
	Attributes *map[string]interface{} `json:"attributes,omitempty"`
}
