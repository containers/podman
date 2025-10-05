//go:build !remote

package server

import (
	"net/http"

	"github.com/containers/podman/v5/pkg/api/handlers/libpod"
	"github.com/gorilla/mux"
)

func (s *APIServer) registerQuadletHandlers(r *mux.Router) error {
	// swagger:operation GET /libpod/quadlets/json libpod QuadletListLibpod
	// ---
	// tags:
	//   - quadlets
	// summary: List quadlets
	// description: Return a list of all quadlets.
	// produces:
	// - application/json
	// parameters:
	//  - in: query
	//    name: filters
	//    type: string
	//    description: |
	//      JSON encoded value of the filters (a map[string][]string).
	//      Supported filters:
	//        - name=<quadlet-name> Filter by quadlet name
	// responses:
	//   200:
	//     $ref: "#/responses/quadletListResponse"
	//   500:
	//     $ref: "#/responses/internalError"
	r.HandleFunc(VersionedPath("/libpod/quadlets/json"), s.APIHandler(libpod.ListQuadlets)).Methods(http.MethodGet)
	return nil
}
