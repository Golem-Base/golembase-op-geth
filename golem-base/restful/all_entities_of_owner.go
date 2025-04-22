package restful

import (
	"mime/multipart"
	"net/http"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity"
	"github.com/ethereum/go-ethereum/golem-base/storageutil/entity/entitiesofowner"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"
)

func RegisterAllEntitiesOfOwner(mux *http.ServeMux, backend *eth.EthAPIBackend) {

	mux.HandleFunc("GET /golembase/all_entities/{owner}", func(w http.ResponseWriter, r *http.Request) {

		owner := r.PathValue("owner")
		if owner == "" {
			http.Error(w, "Owner is required", http.StatusBadRequest)
			return
		}

		stateDb, _, err := backend.StateAndHeaderByNumber(r.Context(), rpc.LatestBlockNumber)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			log.Error("Failed to get state and header", "error", err)
			return
		}

		ownerAddress := common.HexToAddress(owner)

		// Set up multipart writer with random boundary
		mw := multipart.NewWriter(w)
		w.Header().Set("Content-Type", mw.FormDataContentType())

		for key := range entitiesofowner.Iterate(stateDb, ownerAddress) {

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

		mw.Close()

	})

}
