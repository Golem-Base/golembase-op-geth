package eth

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/golem-base/golemtype"
	"github.com/ethereum/go-ethereum/golem-base/sqlstore"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity"
)

// golemBaseAPI offers helper utils
type golemBaseAPI struct {
	arkiv *arkivAPI
}

func NewGolemBaseAPI(eth *Ethereum, store *sqlstore.SQLStore) *golemBaseAPI {
	return &golemBaseAPI{
		arkiv: NewArkivAPI(eth, store),
	}
}

func (api *golemBaseAPI) GetStorageValue(ctx context.Context, key common.Hash) ([]byte, error) {
	q := fmt.Sprintf(`$key = %s`, key)
	entities, err := api.arkiv.QueryEntities(ctx, q, QueryOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	if len(entities) != 1 {
		return nil, fmt.Errorf("failed to execute query: expected one row but got %d", len(entities))
	}

	return entities[0].Value, nil
}

func (api *golemBaseAPI) GetEntityMetaData(ctx context.Context, key common.Hash) (*entity.EntityMetaData, error) {
	metadata, err := api.arkiv.GetEntityMetaData(ctx, key)
	if err != nil {
		return nil, err
	}

	return metadata, nil
}

func (api *golemBaseAPI) GetEntitiesToExpireAtBlock(ctx context.Context, expirationBlock uint64) ([]common.Hash, error) {
	q := fmt.Sprintf(`$expiration = %d`, expirationBlock)
	entities, err := api.arkiv.QueryEntities(ctx, q, QueryOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	results := make([]common.Hash, 0, len(entities))
	for _, entity := range entities {
		results = append(results, entity.Key)
	}

	return results, nil
}

func (api *golemBaseAPI) GetEntitiesForStringAnnotationValue(ctx context.Context, key, value string) ([]common.Hash, error) {
	q := fmt.Sprintf(`%s = "%s"`, key, value)
	entities, err := api.arkiv.QueryEntities(ctx, q, QueryOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	results := make([]common.Hash, 0, len(entities))
	for _, entity := range entities {
		results = append(results, entity.Key)
	}

	return results, nil
}

func (api *golemBaseAPI) GetEntitiesForNumericAnnotationValue(ctx context.Context, key string, value uint64) ([]common.Hash, error) {
	q := fmt.Sprintf(`%s = %d`, key, value)
	entities, err := api.arkiv.QueryEntities(ctx, q, QueryOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	results := make([]common.Hash, 0, len(entities))
	for _, entity := range entities {
		results = append(results, entity.Key)
	}

	return results, nil
}

func (api *golemBaseAPI) QueryEntities(ctx context.Context, req string) ([]golemtype.SearchResult, error) {
	entities, err := api.arkiv.QueryEntities(ctx, req, QueryOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	searchResults := make([]golemtype.SearchResult, 0)

	for _, entity := range entities {
		searchResults = append(searchResults, golemtype.SearchResult{
			Key:   entity.Key,
			Value: entity.Value,
		})
	}

	return searchResults, nil
}

// GetEntityCount returns the total number of entities in the storage.
func (api *golemBaseAPI) GetEntityCount(ctx context.Context) (uint64, error) {
	return api.arkiv.GetEntityCount(ctx)
}

// GetAllEntityKeys returns all entity keys in the storage.
func (api *golemBaseAPI) GetAllEntityKeys(ctx context.Context) ([]common.Hash, error) {
	return api.arkiv.GetAllEntityKeys(ctx)
}

func (api *golemBaseAPI) GetEntitiesOfOwner(ctx context.Context, owner common.Address) ([]common.Hash, error) {
	q := fmt.Sprintf(`$owner = %s`, owner)
	entities, err := api.arkiv.QueryEntities(ctx, q, QueryOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	results := make([]common.Hash, 0, len(entities))
	for _, entity := range entities {
		results = append(results, entity.Key)
	}

	return results, nil
}

func (api *golemBaseAPI) GetNumberOfUsedSlots() (*hexutil.Big, error) {
	return api.arkiv.GetNumberOfUsedSlots()
}
