package server

import (
	"net/http"

	"github.com/containers/podman/v4/pkg/api/handlers/compat"
	"github.com/containers/podman/v4/pkg/api/handlers/libpod"
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
