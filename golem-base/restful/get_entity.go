package restful

import (
	"fmt"
	"net/http"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"
)

func RegisterGetEntity(mux *http.ServeMux, backend *eth.EthAPIBackend) {
	mux.HandleFunc("GET /golembase/entity/{key}", func(w http.ResponseWriter, r *http.Request) {

		key := common.HexToHash(r.PathValue("key"))

		ctx := r.Context()

		stateDb, _, err := backend.StateAndHeaderByNumber(ctx, rpc.LatestBlockNumber)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		size := entity.GetPayloadSize(stateDb, key)

		log.Info("GET /golembase/entity", "key", key, "size", size)

		if size == 0 {
			http.Error(w, "Entity not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", size))

		err = entity.WritePayloadTo(stateDb, key, w)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

	})

}
