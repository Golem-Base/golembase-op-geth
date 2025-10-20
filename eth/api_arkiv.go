package eth

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/golem-base/arkivtype"
	"github.com/ethereum/go-ethereum/golem-base/query"
	"github.com/ethereum/go-ethereum/golem-base/sqlstore"
	"github.com/ethereum/go-ethereum/golem-base/storageaccounting"
)

type IncludeData struct {
	Key         bool `json:"key"`
	Annotations bool `json:"annotations"`
	Payload     bool `json:"payload"`
	Expiration  bool `json:"expiration"`
	Owner       bool `json:"owner"`
}

type QueryOptions struct {
	AtBlock        *uint64      `json:"atBlock"`
	IncludeData    *IncludeData `json:"includeData"`
	ResultsPerPage uint64       `json:"resultsPerPage"`
	Cursor         uint64       `json:"cursor,string"`
}

var allColumns = []string{"key", "expires_at", "owner_address", "payload"}

func (options *QueryOptions) toInternalQueryOptions() internalQueryOptions {

	if options == nil {

	}
	switch {
	case options == nil:
		return internalQueryOptions{
			Columns:            allColumns,
			IncludeAnnotations: true,
		}
	case options.IncludeData == nil:
		return internalQueryOptions{
			Columns:            allColumns,
			IncludeAnnotations: true,
			AtBlock:            options.AtBlock,
			Cursor:             options.Cursor,
		}
	default:
		iq := internalQueryOptions{
			Columns: []string{},
			AtBlock: options.AtBlock,
			Cursor:  options.Cursor,
		}
		if options.IncludeData.Annotations {
			iq.IncludeAnnotations = true
		}
		if options.IncludeData.Payload {
			iq.Columns = append(iq.Columns, "payload")
		}
		if options.IncludeData.Expiration {
			iq.Columns = append(iq.Columns, "expires_at")
		}
		if options.IncludeData.Owner {
			iq.Columns = append(iq.Columns, "owner_address")
		}
		if options.IncludeData.Key {
			iq.Columns = append(iq.Columns, "key")
		}
		return iq
	}
}

type internalQueryOptions struct {
	AtBlock *uint64
	// TODO Ramses: implement this
	// After that we can use it in GetEntityMetaData
	IncludeAnnotations bool
	Columns            []string
	Cursor             uint64
}

type arkivAPI struct {
	eth   *Ethereum
	store *sqlstore.SQLStore
}

func NewArkivAPI(eth *Ethereum, store *sqlstore.SQLStore) *arkivAPI {
	return &arkivAPI{
		eth:   eth,
		store: store,
	}
}

func (api *arkivAPI) Query(
	ctx context.Context,
	req string,
	op *QueryOptions,
) (*arkivtype.QueryResponse, error) {

	expr, err := query.Parse(req)
	if err != nil {
		return nil, fmt.Errorf("failed to parse query: %w", err)
	}

	options := op.toInternalQueryOptions()

	block := api.eth.blockchain.CurrentBlock().Number.Uint64()
	if options.AtBlock != nil {
		block = *options.AtBlock
	}
	columns := query.COLUMNS
	if len(options.Columns) > 0 {
		columns = options.Columns
	}

	queryOptions := query.QueryOptions{
		AtBlock:            block,
		IncludeAnnotations: options.IncludeAnnotations,
		Columns:            columns,
		Offset:             options.Cursor,
	}
	query := expr.Evaluate(queryOptions)

	response := &arkivtype.QueryResponse{
		BlockNumber: block,
		Data:        make([]json.RawMessage, 0),
		Cursor:      0,
	}

	offset := options.Cursor

	// 256 bytes is for the overhead of the 'envelope' around the entity data
	// and the separator characters in between
	responseSize := 256

	// 256 kb
	maxResponseSize := 256 * 1024 * 1024
	maxResultsPerPage := 0

	if op != nil {
		maxResultsPerPage = int(op.ResultsPerPage)
	}

	err = api.store.QueryEntitiesInternalIterator(
		ctx,
		query.Query,
		query.Args,
		queryOptions,
		func(entity arkivtype.EntityData) error {

			ed, err := json.Marshal(entity)
			if err != nil {
				return fmt.Errorf("failed to marshal entity: %w", err)
			}

			newLen := responseSize + len(ed) + 1
			if newLen > maxResponseSize {
				response.Cursor = offset
				return sqlstore.ErrStopIteration
			}
			response.Data = append(response.Data, ed)
			offset++

			if maxResultsPerPage > 0 && len(response.Data) >= maxResultsPerPage {
				response.Cursor = offset
				return sqlstore.ErrStopIteration
			}

			return nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	return response, nil
}

// GetEntityCount returns the total number of entities in the storage.
func (api *arkivAPI) GetEntityCount(ctx context.Context) (uint64, error) {
	count, err := api.store.GetEntityCount(ctx, api.eth.blockchain.CurrentBlock().Number.Uint64())
	if err != nil {
		return 0, err
	}

	return count, nil
}

func (api *arkivAPI) GetNumberOfUsedSlots() (*hexutil.Big, error) {
	header := api.eth.blockchain.CurrentBlock()
	stateDB, err := api.eth.BlockChain().StateAt(header.Root)
	if err != nil {
		return nil, fmt.Errorf("failed to get state: %w", err)
	}

	counter := storageaccounting.GetNumberOfUsedSlots(stateDB)
	counterAsBigInt := big.NewInt(0)
	counter.IntoBig(&counterAsBigInt)
	return (*hexutil.Big)(counterAsBigInt), nil
}

type BlockTiming struct {
	CurrentBlock     uint64 `json:"current_block"`
	CurrentBlockTime uint64 `json:"current_block_time"`
	BlockDuration    uint64 `json:"duration"`
}

func (api *arkivAPI) GetBlockTiming(ctx context.Context) (*BlockTiming, error) {
	header := api.eth.blockchain.CurrentHeader()
	previousHeader := api.eth.blockchain.GetHeaderByHash(header.ParentHash)
	if previousHeader == nil {
		return nil, fmt.Errorf("failed to get previous header")
	}

	return &BlockTiming{
		CurrentBlock:     header.Number.Uint64(),
		CurrentBlockTime: header.Time,
		BlockDuration:    header.Time - previousHeader.Time,
	}, nil
}
