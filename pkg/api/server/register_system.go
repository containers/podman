//go:build !remote

package server

import (
	"net/http"

	"github.com/containers/podman/v5/pkg/api/handlers/compat"
	"github.com/containers/podman/v5/pkg/api/handlers/libpod"
	"github.com/gorilla/mux"
)

func (s *APIServer) registerSystemHandlers(r *mux.Router) error {
	// swagger:operation GET /system/df compat SystemDataUsage
	// ---
	// tags:
	//   - system (compat)
	// summary: Show disk usage
	// description: Return information about disk usage for containers, images, and volumes
	// produces:
	// - application/json
	// responses:
	//   200:
	//     $ref: '#/responses/systemDiskUsage'
	//   500:
	//     $ref: "#/responses/internalError"
	r.Handle(VersionedPath("/system/df"), s.APIHandler(compat.GetDiskUsage)).Methods(http.MethodGet)
	// Added non version path to URI to support docker non versioned paths
	r.Handle("/system/df", s.APIHandler(compat.GetDiskUsage)).Methods(http.MethodGet)
	// swagger:operation POST /libpod/system/check libpod SystemCheckLibpod
	// ---
	// tags:
	//   - system
	// summary: Performs consistency checks on storage, optionally removing items which fail checks
	// parameters:
	//   - in: query
	//     name: quick
	//     type: boolean
	//     description: Skip time-consuming checks
	//   - in: query
	//     name: repair
	//     type: boolean
	//     description: Remove inconsistent images
	//   - in: query
	//     name: repair_lossy
	//     type: boolean
	//     description: Remove inconsistent containers and images
	//   - in: query
	//     name: unreferenced_layer_max_age
	//     type: string
	//     description: Maximum allowed age of unreferenced layers
	//     default: 24h0m0s
	// produces:
	// - application/json
	// responses:
	//   200:
	//     $ref: '#/responses/systemCheckResponse'
	//   400:
	//     $ref: "#/responses/badParamError"
	//   500:
	//     $ref: "#/responses/internalError"
	r.Handle(VersionedPath("/libpod/system/check"), s.APIHandler(libpod.SystemCheck)).Methods(http.MethodPost)
	// swagger:operation POST /libpod/system/prune libpod SystemPruneLibpod
	// ---
	// tags:
	//   - system
	// summary: Prune unused data
	// produces:
	// - application/json
	// responses:
	//   200:
	//     $ref: '#/responses/systemPruneResponse'
	//   400:
	//     $ref: "#/responses/badParamError"
	//   500:
	//     $ref: "#/responses/internalError"
	r.Handle(VersionedPath("/libpod/system/prune"), s.APIHandler(libpod.SystemPrune)).Methods(http.MethodPost)
	// swagger:operation GET /libpod/system/df libpod SystemDataUsageLibpod
	// ---
	// tags:
	//   - system
	// summary: Show disk usage
	// description: Return information about disk usage for containers, images, and volumes
	// produces:
	// - application/json
	// responses:
	//   200:
	//     $ref: '#/responses/systemDiskUsage'
	//   500:
	//     $ref: "#/responses/internalError"
	r.Handle(VersionedPath("/libpod/system/df"), s.APIHandler(libpod.DiskUsage)).Methods(http.MethodGet)
	return nil
}
