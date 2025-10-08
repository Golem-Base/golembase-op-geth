package eth

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/golem-base/arkivtype"
	"github.com/ethereum/go-ethereum/golem-base/query"
	"github.com/ethereum/go-ethereum/golem-base/sqlstore"
	"github.com/ethereum/go-ethereum/golem-base/storageaccounting"
)

type QueryOptions struct {
	AtBlock *uint64 `json:"at_block"`
	// TODO Ramses: implement this
	// After that we can use it in GetEntityMetaData
	IncludeAnnotations bool     `json:"include_annotations"`
	Columns            []string `json:"columns"`
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

func (api *arkivAPI) QueryEntities(
	ctx context.Context,
	req string,
	options QueryOptions,
) ([]arkivtype.SearchResult, error) {

	expr, err := query.Parse(req)
	if err != nil {
		return nil, fmt.Errorf("failed to parse query: %w", err)
	}

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
	}
	query := expr.Evaluate(queryOptions)

	results, err := api.store.QueryEntities(
		ctx,
		query.Query,
		query.Args,
		queryOptions,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	return results, nil
}

// GetEntityCount returns the total number of entities in the storage.
func (api *arkivAPI) GetEntityCount(ctx context.Context) (uint64, error) {
	count, err := api.store.GetEntityCount(ctx, api.eth.blockchain.CurrentBlock().Number.Uint64())
	if err != nil {
		return 0, err
	}

	return count, nil
}

// GetAllEntityKeys returns all entity keys in the storage.
func (api *arkivAPI) GetAllEntityKeys(ctx context.Context) ([]common.Hash, error) {
	entities, err := api.store.GetAllEntityKeys(ctx, api.eth.blockchain.CurrentBlock().Number.Uint64())
	if err != nil {
		return nil, err
	}

	return entities, nil
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
