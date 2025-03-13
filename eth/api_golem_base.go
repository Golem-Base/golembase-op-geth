package eth

import (
	"encoding/binary"
	"fmt"
	"slices"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/golem-base/golemtype"
	"github.com/ethereum/go-ethereum/golem-base/query"
	"github.com/ethereum/go-ethereum/golem-base/storageutil"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/keyset"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/holiman/uint256"
)

// golemBaseAPI offers helper utils
type golemBaseAPI struct {
	eth *Ethereum
}

func NewGolemBaseAPI(eth *Ethereum) *golemBaseAPI {
	return &golemBaseAPI{
		eth: eth,
	}
}

func (api *golemBaseAPI) GetStorageValue(key common.Hash) ([]byte, error) {
	header := api.eth.blockchain.CurrentBlock()
	stateDb, err := api.eth.BlockChain().StateAt(header.Root)
	if err != nil {
		return nil, err
	}

	v := storageutil.GetGolemDBState(stateDb, key)

	ap := storageutil.ActivePayload{}

	err = rlp.DecodeBytes(v, &ap)
	if err != nil {
		return nil, fmt.Errorf("failed to decode active payload: %w", err)
	}

	return ap.Payload, nil
}

func (api *golemBaseAPI) GetEntitiesToExpireAtBlock(blockNumber uint64) ([]common.Hash, error) {
	header := api.eth.blockchain.CurrentBlock()
	stateDb, err := api.eth.BlockChain().StateAt(header.Root)
	if err != nil {
		return nil, err
	}

	blockNumberBig := uint256.NewInt(blockNumber)

	expiredEntityKey := crypto.Keccak256Hash([]byte("golemBaseExpiresAtBlock"), blockNumberBig.Bytes())

	return slices.Collect(keyset.Iterate(stateDb, expiredEntityKey)), nil
}

func (api *golemBaseAPI) GetEntitiesForStringAnnotationValue(key, value string) ([]common.Hash, error) {
	header := api.eth.blockchain.CurrentBlock()
	stateDb, err := api.eth.BlockChain().StateAt(header.Root)
	if err != nil {
		return nil, err
	}

	entityKeys := crypto.Keccak256Hash(
		[]byte("golemBaseStringAnnotation"),
		[]byte(key),
		[]byte(value),
	)

	return slices.Collect(keyset.Iterate(stateDb, entityKeys)), nil
}

func (api *golemBaseAPI) GetEntitiesForNumericAnnotationValue(key string, value uint64) ([]common.Hash, error) {
	header := api.eth.blockchain.CurrentBlock()
	stateDb, err := api.eth.BlockChain().StateAt(header.Root)
	if err != nil {
		return nil, err
	}

	entityKeys := crypto.Keccak256Hash(
		[]byte("golemBaseNumericAnnotation"),
		[]byte(key),
		binary.BigEndian.AppendUint64(nil, value),
	)

	return slices.Collect(keyset.Iterate(stateDb, entityKeys)), nil
}

func (api *golemBaseAPI) QueryEntities(req string) ([]golemtype.SearchResult, error) {

	expr, err := query.Parse(req)
	if err != nil {
		return nil, fmt.Errorf("failed to parse query: %w", err)
	}

	ds := &golemBaseDataSource{api: api}
	entites, err := expr.Evaluate(ds)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate query: %w", err)
	}

	var searchResults []golemtype.SearchResult

	for _, key := range entites {
		v, err := api.GetStorageValue(key)
		if err != nil {
			return nil, fmt.Errorf("failed to get storage value for key %s: %w", key.Hex(), err)
		}
		searchResults = append(searchResults, golemtype.SearchResult{
			Key:   key,
			Value: v,
		})
	}

	return searchResults, nil

}

type golemBaseDataSource struct {
	api *golemBaseAPI
}

func (ds *golemBaseDataSource) GetKeysForStringAnnotation(key, value string) ([]common.Hash, error) {
	return ds.api.GetEntitiesForStringAnnotationValue(key, value)
}

func (ds *golemBaseDataSource) GetKeysForNumericAnnotation(key string, value uint64) ([]common.Hash, error) {
	return ds.api.GetEntitiesForNumericAnnotationValue(key, value)
}
