package server

import (
	"net/http"

	"github.com/containers/libpod/pkg/api/handlers/compat"
	"github.com/containers/libpod/pkg/api/handlers/libpod"
	"github.com/gorilla/mux"
)

func (s *APIServer) registerSystemHandlers(r *mux.Router) error {
	r.Handle(VersionedPath("/system/df"), s.APIHandler(compat.GetDiskUsage)).Methods(http.MethodGet)
	// Added non version path to URI to support docker non versioned paths
	r.Handle("/system/df", s.APIHandler(compat.GetDiskUsage)).Methods(http.MethodGet)
	// swagger:operation POST /libpod/system/prune libpod pruneSystem
	// ---
	// tags:
	//   - system
	// summary: Prune unused data
	// produces:
	// - application/json
	// responses:
	//   200:
	//     $ref: '#/responses/SystemPruneReport'
	//   400:
	//     $ref: "#/responses/BadParamError"
	//   500:
	//     $ref: "#/responses/InternalError"
	r.Handle(VersionedPath("/libpod/system/prune"), s.APIHandler(libpod.SystemPrune)).Methods(http.MethodPost)
	// swagger:operation POST /libpod/system/reset libpod resetSystem
	// ---
	// tags:
	//   - system
	// summary: Reset podman storage
	// description: All containers will be stopped and removed, and all images, volumes and container content will be removed.
	// produces:
	// - application/json
	// responses:
	//   200:
	//     description: no error
	//   500:
	//     $ref: "#/responses/InternalError"
	r.Handle(VersionedPath("/libpod/system/reset"), s.APIHandler(libpod.SystemReset)).Methods(http.MethodPost)
	// swagger:operation GET /libpod/system/df libpod df
	// ---
	// tags:
	//   - system
	// summary: Show disk usage
	// description: Return information about disk usage for containers, images, and volumes
	// produces:
	// - application/json
	// responses:
	//   200:
	//     $ref: '#/responses/SystemDiskUse'
	//   500:
	//     $ref: "#/responses/InternalError"
	r.Handle(VersionedPath("/libpod/system/df"), s.APIHandler(libpod.DiskUsage)).Methods(http.MethodGet)
	return nil
}
