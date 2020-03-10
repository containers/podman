package server

import (
	"net/http"

	"github.com/containers/libpod/pkg/api/handlers/compat"
	"github.com/gorilla/mux"
)

func (s *APIServer) registerInfoHandlers(r *mux.Router) error {
	// swagger:operation GET /info libpod libpodGetInfo
	// ---
	// tags:
	//  - system
	// summary: Get info
	// description: Returns information on the system and libpod configuration
	// produces:
	// - application/json
	// responses:
	//   200:
	//     description: to be determined
	//   500:
	//     $ref: "#/responses/InternalError"
	r.Handle(VersionedPath("/info"), s.APIHandler(compat.GetInfo)).Methods(http.MethodGet)
	// Added non version path to URI to support docker non versioned paths
	r.Handle("/info", s.APIHandler(compat.GetInfo)).Methods(http.MethodGet)
	return nil
}
