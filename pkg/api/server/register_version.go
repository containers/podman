//go:build !remote

package server

import (
	"net/http"

	"github.com/containers/podman/v5/pkg/api/handlers/compat"
	"github.com/gorilla/mux"
)

func (s *APIServer) registerVersionHandlers(r *mux.Router) error {
	// swagger:operation GET /version compat SystemVersion
	// ---
	// summary: Component Version information
	// tags:
	// - system (compat)
	// produces:
	// - application/json
	// responses:
	//   200:
	//    $ref: "#/responses/versionResponse"
	r.Handle("/version", s.APIHandler(compat.VersionHandler)).Methods(http.MethodGet)
	r.Handle(VersionedPath("/version"), s.APIHandler(compat.VersionHandler)).Methods(http.MethodGet)
	// swagger:operation GET /libpod/version libpod SystemVersionLibpod
	// ---
	// summary: Component Version information
	// tags:
	// - system
	// produces:
	// - application/json
	// responses:
	//   200:
	//    $ref: "#/responses/versionResponse"
	r.Handle(VersionedPath("/libpod/version"), s.APIHandler(compat.VersionHandler)).Methods(http.MethodGet)
	return nil
}
