package server

import (
	"net/http"

	"github.com/containers/libpod/pkg/api/handlers/libpod"
	"github.com/gorilla/mux"
)

func (s *APIServer) registerNetworkHandlers(r *mux.Router) error {
	// swagger:operation DELETE /libpod/networks/{name} libpod libpodRemoveNetwork
	// ---
	// tags:
	//  - networks
	// summary: Remove a network
	// description: Remove a CNI configured network
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name of the network
	//  - in: query
	//    name: Force
	//    type: boolean
	//    description: remove containers associated with network
	// produces:
	// - application/json
	// responses:
	//   200:
	//     $ref: "#/responses/NetworkRmReport"
	//   404:
	//     $ref: "#/responses/NoSuchNetwork"
	//   500:
	//     $ref: "#/responses/InternalError"
	r.HandleFunc(VersionedPath("/libpod/networks/{name}"), s.APIHandler(libpod.RemoveNetwork)).Methods(http.MethodDelete)
	// swagger:operation GET /libpod/networks/{name}/json libpod libpodInspectNetwork
	// ---
	// tags:
	//  - networks
	// summary: Inspect a network
	// description: Display low level configuration for a CNI network
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name of the network
	// produces:
	// - application/json
	// responses:
	//   200:
	//     $ref: "#/responses/NetworkInspectReport"
	//   404:
	//     $ref: "#/responses/NoSuchNetwork"
	//   500:
	//     $ref: "#/responses/InternalError"
	r.HandleFunc(VersionedPath("/libpod/networks/{name}/json"), s.APIHandler(libpod.InspectNetwork)).Methods(http.MethodGet)
	// swagger:operation GET /libpod/networks/json libpod libpodListNetwork
	// ---
	// tags:
	//  - networks
	// summary: List networks
	// description: Display summary of network configurations
	// produces:
	// - application/json
	// responses:
	//   200:
	//     $ref: "#/responses/NetworkListReport"
	//   500:
	//     $ref: "#/responses/InternalError"
	r.HandleFunc(VersionedPath("/libpod/networks/json"), s.APIHandler(libpod.ListNetworks)).Methods(http.MethodGet)
	// swagger:operation POST /libpod/networks/create libpod libpodCreateNetwork
	// ---
	// tags:
	//  - networks
	// summary: Create network
	// description: Create a new CNI network configuration
	// produces:
	// - application/json
	// parameters:
	//  - in: query
	//    name: name
	//    type: string
	//    description: optional name for new network
	//  - in: body
	//    name: create
	//    description: attributes for creating a container
	//    schema:
	//      $ref: "#/definitions/NetworkCreateOptions"
	// responses:
	//   200:
	//     $ref: "#/responses/NetworkCreateReport"
	//   400:
	//     $ref: "#/responses/BadParamError"
	//   500:
	//     $ref: "#/responses/InternalError"
	r.HandleFunc(VersionedPath("/libpod/networks/create"), s.APIHandler(libpod.CreateNetwork)).Methods(http.MethodPost)
	return nil
}
