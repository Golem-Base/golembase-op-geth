package eth

import (
	"context"

	"github.com/ethereum/go-ethereum/golem-base/sqlstore"
)

// golemBaseAPI offers helper utils
type golemDBAPI struct {
	eth   *Ethereum
	store *sqlstore.SQLStore
}

func NewGolemDBAPI(eth *Ethereum, store *sqlstore.SQLStore) *golemDBAPI {
	return &golemDBAPI{
		eth:   eth,
		store: store,
	}
}

func (api *golemDBAPI) Query(ctx context.Context, q string, from *uint64) ([]byte, error) {

	return nil, nil

}
