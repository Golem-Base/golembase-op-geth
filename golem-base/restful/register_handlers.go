package restful

import (
	"net/http"

	"github.com/ethereum/go-ethereum/eth"
)

func RegisterHandlers(mux *http.ServeMux, backend *eth.EthAPIBackend) {
	RegisterGetEntity(mux, backend)
	RegisterQuery(mux, backend)
}
