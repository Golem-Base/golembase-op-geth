package arkivtype

import (
	"encoding/json"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

var KeyAttributeKey = "$key"
var CreatorAttributeKey = "$creator"
var OwnerAttributeKey = "$owner"
var ExpirationAttributeKey = "$expiration"
var SequenceAttributeKey = "$sequence"

var allColumns = []string{
	"key",
	"payload",
	"content_type",
	"expires_at",
	"owner_address",
	"created_at_block",
	"last_modified_at_block",
	"transaction_index_in_block",
	"operation_index_in_transaction",
}

type columnEntry struct {
	Index int
	Name  string
}

// allColumnsMapping is used to verify user-supplied columns to protect against SQL injection
var allColumnsMapping = make(map[string]columnEntry)

func init() {
	for i, column := range allColumns {
		allColumnsMapping[column] = columnEntry{
			Index: i,
			Name:  column,
		}
	}
}

func getColumn(name string) (*columnEntry, error) {
	column, exists := allColumnsMapping[name]
	if !exists {
		return nil, fmt.Errorf("invalid column name: %s", column.Name)
	}
	return &column, nil
}

func GetColumn(name string) (string, error) {
	column, err := getColumn(name)
	if err != nil {
		return "", err
	}
	return column.Name, nil
}

// GetColumnOrPanic is used for non user-supplied columns, to detect wrong literals early
func GetColumnOrPanic(name string) string {
	column, err := getColumn(name)
	if err != nil {
		panic(fmt.Sprintf("invalid column name: %s", column.Name))
	}
	return column.Name
}

type OrderByAnnotation struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Descending bool   `json:"desc"`
}

type QueryResponse struct {
	Data        []json.RawMessage `json:"data"`
	BlockNumber hexutil.Uint64    `json:"blockNumber"`
	Cursor      *string           `json:"cursor,omitempty"`
}

type Cursor struct {
	BlockNumber  uint64        `json:"blockNumber"`
	ColumnValues []CursorValue `json:"columnValues"`
}

type CursorValue struct {
	ColumnName string `json:"columnName"`
	Value      any    `json:"value"`
	Descending bool   `json:"desc"`
}

type EntityData struct {
	Key                         *common.Hash    `json:"key,omitempty"`
	Value                       hexutil.Bytes   `json:"value,omitempty"`
	ContentType                 *string         `json:"contentType,omitempty"`
	ExpiresAt                   *hexutil.Uint64 `json:"expiresAt,omitempty"`
	Owner                       *common.Address `json:"owner,omitempty"`
	CreatedAtBlock              *hexutil.Uint64 `json:"createdAtBlock,omitempty"`
	LastModifiedAtBlock         *hexutil.Uint64 `json:"lastModifiedAtBlock,omitempty"`
	TransactionIndexInBlock     *hexutil.Uint64 `json:"transactionIndexInBlock,omitempty"`
	OperationIndexInTransaction *hexutil.Uint64 `json:"operationIndexInTransaction,omitempty"`

	StringAttributes  []StringAnnotation  `json:"stringAttributes,omitempty"`
	NumericAttributes []NumericAnnotation `json:"numericAttributes,omitempty"`
}

type StringAnnotation struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type NumericAnnotation struct {
	Key   string         `json:"key"`
	Value hexutil.Uint64 `json:"value"`
}
