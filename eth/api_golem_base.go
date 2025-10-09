package eth

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/golem-base/arkivtype"
	"github.com/ethereum/go-ethereum/golem-base/golemtype"
	"github.com/ethereum/go-ethereum/golem-base/sqlstore"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity"
)

// golemBaseAPI offers helper utils
type golemBaseAPI struct {
	*arkivAPI
}

func NewGolemBaseAPI(eth *Ethereum, store *sqlstore.SQLStore) *golemBaseAPI {
	return &golemBaseAPI{
		arkivAPI: NewArkivAPI(eth, store),
	}
}

func (api *golemBaseAPI) GetStorageValue(ctx context.Context, key common.Hash) ([]byte, error) {
	q := fmt.Sprintf(`$key = %s`, key)

	result, err := api.arkivAPI.QueryEntities(
		ctx,
		q,
		arkivtype.QueryOptions{
			Columns: []string{"payload"},
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	entities := result.Results

	if len(entities) != 1 {
		return nil, fmt.Errorf("expected a single result but got %d", len(entities))
	}

	return entities[0].Value, nil
}

func (api *golemBaseAPI) GetEntityMetaData(ctx context.Context, key common.Hash) (*entity.EntityMetaData, error) {
	result, err := api.arkivAPI.QueryEntities(
		ctx,
		fmt.Sprintf("$key = %s", key),
		arkivtype.QueryOptions{
			IncludeAnnotations: true,
		},
	)
	if err != nil {
		return nil, err
	}

	rows := result.Results

	if len(rows) != 1 {
		return nil, fmt.Errorf("expected a single result row but got %d", len(rows))
	}

	metadata := rows[0]

	return &entity.EntityMetaData{
		ExpiresAtBlock:     metadata.ExpiresAt,
		Owner:              metadata.Owner,
		StringAnnotations:  metadata.StringAnnotations,
		NumericAnnotations: metadata.NumericAnnotations,
	}, nil
}

func (api *golemBaseAPI) GetEntitiesToExpireAtBlock(ctx context.Context, expirationBlock uint64) ([]common.Hash, error) {
	q := fmt.Sprintf(`$expiration = %d`, expirationBlock)
	result, err := api.arkivAPI.QueryEntities(ctx, q, arkivtype.QueryOptions{
		Columns: []string{"key"},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	entities := result.Results

	results := make([]common.Hash, 0, len(entities))
	for _, entity := range entities {
		results = append(results, entity.Key)
	}

	return results, nil
}

func (api *golemBaseAPI) GetEntitiesForStringAnnotationValue(ctx context.Context, key, value string) ([]common.Hash, error) {
	q := fmt.Sprintf(`%s = "%s"`, key, value)
	result, err := api.arkivAPI.QueryEntities(ctx, q, arkivtype.QueryOptions{
		Columns: []string{"key"},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	entities := result.Results

	results := make([]common.Hash, 0, len(entities))
	for _, entity := range entities {
		results = append(results, entity.Key)
	}

	return results, nil
}

func (api *golemBaseAPI) GetEntitiesForNumericAnnotationValue(ctx context.Context, key string, value uint64) ([]common.Hash, error) {
	q := fmt.Sprintf(`%s = %d`, key, value)
	result, err := api.arkivAPI.QueryEntities(ctx, q, arkivtype.QueryOptions{
		Columns: []string{"key"},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	entities := result.Results

	results := make([]common.Hash, 0, len(entities))
	for _, entity := range entities {
		results = append(results, entity.Key)
	}

	return results, nil
}

func (api *golemBaseAPI) QueryEntities(ctx context.Context, req string) ([]golemtype.SearchResult, error) {
	result, err := api.arkivAPI.QueryEntities(ctx, req, arkivtype.QueryOptions{
		Columns: []string{"key", "payload"},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	searchResults := make([]golemtype.SearchResult, 0)

	entities := result.Results

	for _, entity := range entities {
		searchResults = append(searchResults, golemtype.SearchResult{
			Key:   entity.Key,
			Value: entity.Value,
		})
	}

	api.GetEntityCount(ctx)

	return searchResults, nil
}

func (api *golemBaseAPI) GetEntitiesOfOwner(ctx context.Context, owner common.Address) ([]common.Hash, error) {
	q := fmt.Sprintf(`$owner = %s`, owner)
	result, err := api.arkivAPI.QueryEntities(ctx, q, arkivtype.QueryOptions{
		Columns: []string{"key"},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	entities := result.Results

	results := make([]common.Hash, 0, len(entities))
	for _, entity := range entities {
		results = append(results, entity.Key)
	}

	return results, nil
}

// GetEntityCount returns the total number of entities in the storage.
func (api *golemBaseAPI) GetEntityCount(ctx context.Context) (uint64, error) {
	count, err := api.store.GetEntityCount(ctx, api.eth.blockchain.CurrentBlock().Number.Uint64())
	if err != nil {
		return 0, err
	}

	return count, nil
}

// GetAllEntityKeys returns all entity keys in the storage.
func (api *golemBaseAPI) GetAllEntityKeys(ctx context.Context) ([]common.Hash, error) {
	entities, err := api.store.GetAllEntityKeys(ctx, api.eth.blockchain.CurrentBlock().Number.Uint64())
	if err != nil {
		return nil, err
	}

	return entities, nil
}
