package server

import (
	"net/http"

	"github.com/containers/libpod/pkg/api/handlers/libpod"
	"github.com/gorilla/mux"
)

func (s *APIServer) registerGenerateHandlers(r *mux.Router) error {
	// swagger:operation GET /libpod/generate/{name:.*}/kube libpod libpodGenerateKube
	// ---
	// tags:
	//  - containers
	//  - pods
	// summary: Play a Kubernetes YAML file.
	// description: Create and run pods based on a Kubernetes YAML file (pod or service kind).
	// parameters:
	//  - in: path
	//    name: name:.*
	//    type: string
	//    required: true
	//    description: Name or ID of the container or pod.
	//  - in: query
	//    name: service
	//    type: boolean
	//    default: false
	//    description: Generate YAML for a Kubernetes service object.
	// produces:
	// - application/json
	// responses:
	//   200:
	//     description: no error
	//     schema:
	//      type: string
	//      format: binary
	//   500:
	//     $ref: "#/responses/InternalError"
	r.HandleFunc(VersionedPath("/libpod/generate/{name:.*}/kube"), s.APIHandler(libpod.GenerateKube)).Methods(http.MethodGet)
	return nil
}
