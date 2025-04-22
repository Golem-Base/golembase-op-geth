package restful

import (
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"slices"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/golem-base/query"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity/annotationindex"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/keyset"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"
)

type Query struct {
	Query string `json:"query"`
}

type queryDataSource struct {
	ctx     context.Context
	stateDb *state.StateDB
}

func (ds queryDataSource) GetKeysForStringAnnotation(key, value string) ([]common.Hash, error) {

	entitySetKey := annotationindex.StringAnnotationIndexKey(key, value)

	return slices.Collect(keyset.Iterate(ds.stateDb, entitySetKey)), nil

}

func (ds queryDataSource) GetKeysForNumericAnnotation(key string, value uint64) ([]common.Hash, error) {

	entitySetKey := annotationindex.NumericAnnotationIndexKey(key, value)

	return slices.Collect(keyset.Iterate(ds.stateDb, entitySetKey)), nil

}

func RegisterQuery(mux *http.ServeMux, backend *eth.EthAPIBackend) {

	mux.HandleFunc("POST /golembase/query", func(w http.ResponseWriter, r *http.Request) {

		q := Query{}
		err := json.NewDecoder(r.Body).Decode(&q)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			log.Error("Failed to decode query", "error", err)
			return
		}

		stateDb, _, err := backend.StateAndHeaderByNumber(r.Context(), rpc.LatestBlockNumber)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			log.Error("Failed to get state and header", "error", err)
			return
		}

		ds := queryDataSource{
			ctx:     r.Context(),
			stateDb: stateDb,
		}

		expr, err := query.Parse(q.Query)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			log.Error("Failed to parse query", "error", err)
			return
		}

		entites, err := expr.Evaluate(ds)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			log.Error("Failed to evaluate query", "error", err)
			return
		}

		// Set up multipart writer with random boundary
		mw := multipart.NewWriter(w)
		w.Header().Set("Content-Type", mw.FormDataContentType())

		for _, key := range entites {

			// Create a part for the entity
			partWriter, err := mw.CreateFormFile("entity", key.Hex())
			if err != nil {
				log.Error("Failed to create form file", "error", err)
				break
			}

			err = entity.WritePayloadTo(stateDb, key, partWriter)
			if err != nil {
				log.Error("Failed to write entity payload", "key", key, "error", err)
				break
			}

		}

		// Close the multipart writer to finalize the response
		mw.Close()
	})

}
