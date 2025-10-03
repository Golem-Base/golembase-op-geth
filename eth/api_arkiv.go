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
	"github.com/ethereum/go-ethereum/golem-base/sqlstore/sqlitegolem"
	"github.com/ethereum/go-ethereum/golem-base/storageaccounting"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity"
)

// arkivAPI offers helper utils
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

func (api *arkivAPI) GetEntityMetaData(ctx context.Context, key common.Hash) (*entity.EntityMetaData, error) {
	metadata, err := api.store.GetEntityMetaData(ctx, sqlitegolem.GetEntityMetadataParams{
		Key:   key.Hex(),
		Block: api.eth.blockchain.CurrentBlock().Number.Int64(),
	})
	if err != nil {
		return nil, err
	}

	return metadata, nil
}

type QueryOptions struct {
	AtBlock            *uint64 `json:"at_block"`
	IncludeAnnotations bool    `json:"include_annotations"`
	Count              bool    `json:"count"`
}

func (api *arkivAPI) QueryEntities(ctx context.Context, req string, options QueryOptions) ([]arkivtype.SearchResult, error) {

	expr, err := query.Parse(req)
	if err != nil {
		return nil, fmt.Errorf("failed to parse query: %w", err)
	}

	block := api.eth.blockchain.CurrentBlock().Number.Uint64()
	if options.AtBlock != nil {
		block = *options.AtBlock
	}

	query := expr.Evaluate(block)

	entities, err := api.store.QueryEntities(ctx, query.Query, query.Args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	return entities, nil
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
