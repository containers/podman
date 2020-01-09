package server

import (
	"net/http"

	"github.com/containers/libpod/pkg/api/handlers/libpod"
	"github.com/gorilla/mux"
)

func (s *APIServer) registerVolumeHandlers(r *mux.Router) error {
	// swagger:operation POST /libpod/volumes/create volumes createVolume
	//
	//  Create a volume
	//
	// ---
	// produces:
	// - application/json
	// responses:
	//   '200':
	//     description: tbd
	//   '500':
	//      "$ref": "#/responses/InternalError"
	r.Handle("/libpod/volumes/create", APIHandler(s.Context, libpod.CreateVolume)).Methods(http.MethodPost)
	r.Handle("/libpod/volumes/json", APIHandler(s.Context, libpod.ListVolumes)).Methods(http.MethodGet)
	// swagger:operation POST /volumes/prune volumes pruneVolumes
	//
	//  Prune volumes
	//
	// ---
	// produces:
	// - application/json
	// responses:
	//   '204':
	//     description: no error
	//   '500':
	//      "$ref": "#/responses/InternalError"
	r.Handle("/libpod/volumes/prune", APIHandler(s.Context, libpod.PruneVolumes)).Methods(http.MethodPost)
	// swagger:operation GET /volumes/{nameOrID}/json volumes inspectVolume
	//
	//  Inspect volume
	//
	// ---
	// parameters:
	//  - in: path
	//    name: nameOrID
	//    required: true
	//    description: the name or ID of the volume
	// produces:
	// - application/json
	// responses:
	//   '200':
	//       "$ref": "#/responses/InspectVolumeResponse"
	//   '404':
	//       "$ref": "#/responses/NoSuchVolume"
	//   '500':
	//      "$ref": "#/responses/InternalError"
	r.Handle("/libpod/volumes/{name:..*}/json", APIHandler(s.Context, libpod.InspectVolume)).Methods(http.MethodGet)
	// swagger:operation DELETE /volumes/{nameOrID} volumes removeVolume
	//
	//  Inspect volume
	//
	// ---
	// parameters:
	//  - in: path
	//    name: nameOrID
	//    required: true
	//    description: the name or ID of the volume
	//  - in: query
	//    name: force
	//    type: bool
	//    description: force removal
	// produces:
	// - application/json
	// responses:
	//   '204':
	//       description: no error
	//   '400':
	//       "$ref": "#/responses/BadParamError"
	//   '404':
	//       "$ref": "#/responses/NoSuchVolume"
	//   '500':
	//      "$ref": "#/responses/InternalError"
	r.Handle("/libpod/volumes/{name:..*}", APIHandler(s.Context, libpod.RemoveVolume)).Methods(http.MethodDelete)
	return nil
}
