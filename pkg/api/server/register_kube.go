package server

import (
	"net/http"

	"github.com/containers/podman/v4/pkg/api/handlers/libpod"
	"github.com/gorilla/mux"
)

func (s *APIServer) registerKubeHandlers(r *mux.Router) error {
	// swagger:operation POST /libpod/play/kube libpod PlayKubeLibpod
	// ---
	// tags:
	//  - containers
	//  - pods
	// summary: Play a Kubernetes YAML file.
	// description: Create and run pods based on a Kubernetes YAML file (pod or service kind).
	// parameters:
	//  - in: query
	//    name: network
	//    type: array
	//    description: USe the network mode or specify an array of networks.
	//    items:
	//      type: string
	//  - in: query
	//    name: tlsVerify
	//    type: boolean
	//    default: true
	//    description: Require HTTPS and verify signatures when contacting registries.
	//  - in: query
	//    name: logDriver
	//    type: string
	//    description: Logging driver for the containers in the pod.
	//  - in: query
	//    name: start
	//    type: boolean
	//    default: true
	//    description: Start the pod after creating it.
	//  - in: query
	//    name: staticIPs
	//    type: array
	//    description: Static IPs used for the pods.
	//    items:
	//      type: string
	//  - in: query
	//    name: staticMACs
	//    type: array
	//    description: Static MACs used for the pods.
	//    items:
	//      type: string
	//  - in: body
	//    name: request
	//    description: Kubernetes YAML file.
	//    schema:
	//      type: string
	// produces:
	// - application/json
	// responses:
	//   200:
	//     $ref: "#/responses/playKubeResponseLibpod"
	//   500:
	//     $ref: "#/responses/internalError"
	r.HandleFunc(VersionedPath("/libpod/play/kube"), s.APIHandler(libpod.PlayKube)).Methods(http.MethodPost)
	r.HandleFunc(VersionedPath("/libpod/kube/play"), s.APIHandler(libpod.KubePlay)).Methods(http.MethodPost)
	// swagger:operation DELETE /libpod/play/kube libpod PlayKubeDownLibpod
	// ---
	// tags:
	//  - containers
	//  - pods
	// summary: Remove pods from kube play
	// description: Tears down pods defined in a YAML file
	// produces:
	// - application/json
	// responses:
	//   200:
	//     $ref: "#/responses/playKubeResponseLibpod"
	//   500:
	//     $ref: "#/responses/internalError"
	r.HandleFunc(VersionedPath("/libpod/play/kube"), s.APIHandler(libpod.PlayKubeDown)).Methods(http.MethodDelete)
	r.HandleFunc(VersionedPath("/libpod/kube/play"), s.APIHandler(libpod.KubePlayDown)).Methods(http.MethodDelete)
	// swagger:operation GET /libpod/generate/kube libpod GenerateKubeLibpod
	// ---
	// tags:
	//  - containers
	//  - pods
	// summary: Generate a Kubernetes YAML file.
	// description: Generate Kubernetes YAML based on a pod or container.
	// parameters:
	//  - in: query
	//    name: names
	//    type: array
	//    items:
	//       type: string
	//    required: true
	//    description: Name or ID of the container or pod.
	//  - in: query
	//    name: service
	//    type: boolean
	//    default: false
	//    description: Generate YAML for a Kubernetes service object.
	// produces:
	// - text/vnd.yaml
	// - application/json
	// responses:
	//   200:
	//     description: Kubernetes YAML file describing pod
	//     schema:
	//      type: string
	//      format: binary
	//   500:
	//     $ref: "#/responses/internalError"
	r.HandleFunc(VersionedPath("/libpod/generate/kube"), s.APIHandler(libpod.GenerateKube)).Methods(http.MethodGet)
	r.HandleFunc(VersionedPath("/libpod/kube/generate"), s.APIHandler(libpod.KubeGenerate)).Methods(http.MethodGet)
	return nil
}
