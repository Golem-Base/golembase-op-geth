package eth

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/golem-base/arkivtype"
	"github.com/ethereum/go-ethereum/golem-base/query"
	"github.com/ethereum/go-ethereum/golem-base/sqlstore"
	"github.com/ethereum/go-ethereum/golem-base/storageaccounting"
)

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
	options arkivtype.QueryOptions,
) (*arkivtype.QueryEntitiesResult, error) {

	expr, err := query.Parse(req)
	if err != nil {
		return nil, fmt.Errorf("failed to parse query: %w", err)
	}

	block := api.eth.blockchain.CurrentBlock().Number.Uint64()
	if options.AtBlock != nil {
		block = *options.AtBlock
	}

	queryOptions, err := query.NewQueryOptions(
		block,
		options.IncludeAnnotations,
		options.Columns,
		options.From,
		options.OrderBy,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate query: %w", err)
	}
	query, err := expr.Evaluate(queryOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate query: %w", err)
	}

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
