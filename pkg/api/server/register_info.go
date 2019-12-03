package server

import (
	"github.com/containers/libpod/pkg/api/handlers"
	"github.com/gorilla/mux"
)

func (s *APIServer) registerInfoHandlers(r *mux.Router) error {
	// swagger:operation GET /info libpod getInfo
	//
	// Returns information on the system and libpod configuration
	//
	// ---
	// produces:
	// - application/json
	// parameters:
	// responses:
	//   '200':
	//		schema:
	//		  "$ref": "#/types/Info"
	//	 '500':
	//   	description: unexpected error
	//		schema:
	//	      "$ref": "#/types/ErrorModel"
	r.Handle(VersionedPath("/info"), APIHandler(s.Context, handlers.GetInfo)).Methods("GET")
	return nil
}
