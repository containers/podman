package server

import (
	"net/http"

	"github.com/containers/podman/v2/pkg/api/handlers/compat"
	"github.com/containers/podman/v2/pkg/api/handlers/libpod"
	"github.com/gorilla/mux"
)

func (s *APIServer) registerNetworkHandlers(r *mux.Router) error {
	// swagger:operation DELETE /networks/{name} compat compatRemoveNetwork
	// ---
	// tags:
	//  - networks (compat)
	// summary: Remove a network
	// description: Remove a network
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name of the network
	// produces:
	// - application/json
	// responses:
	//   204:
	//     description: no error
	//   404:
	//     $ref: "#/responses/NoSuchNetwork"
	//   500:
	//     $ref: "#/responses/InternalError"
	r.HandleFunc(VersionedPath("/networks/{name}"), s.APIHandler(compat.RemoveNetwork)).Methods(http.MethodDelete)
	r.HandleFunc("/networks/{name}", s.APIHandler(compat.RemoveNetwork)).Methods(http.MethodDelete)
	// swagger:operation GET /networks/{name} compat compatInspectNetwork
	// ---
	// tags:
	//  - networks (compat)
	// summary: Inspect a network
	// description: Display low level configuration network
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
	//     $ref: "#/responses/CompatNetworkInspect"
	//   404:
	//     $ref: "#/responses/NoSuchNetwork"
	//   500:
	//     $ref: "#/responses/InternalError"
	r.HandleFunc(VersionedPath("/networks/{name}"), s.APIHandler(compat.InspectNetwork)).Methods(http.MethodGet)
	r.HandleFunc("/networks/{name}", s.APIHandler(compat.InspectNetwork)).Methods(http.MethodGet)
	// swagger:operation GET /networks compat compatListNetwork
	// ---
	// tags:
	//  - networks (compat)
	// summary: List networks
	// description: Display summary of network configurations
	// parameters:
	//  - in: query
	//    name: filters
	//    type: string
	//    description: JSON encoded value of the filters (a map[string][]string) to process on the networks list. Only the name filter is supported.
	// produces:
	// - application/json
	// responses:
	//   200:
	//     $ref: "#/responses/CompatNetworkList"
	//   500:
	//     $ref: "#/responses/InternalError"
	r.HandleFunc(VersionedPath("/networks"), s.APIHandler(compat.ListNetworks)).Methods(http.MethodGet)
	r.HandleFunc("/networks", s.APIHandler(compat.ListNetworks)).Methods(http.MethodGet)
	// swagger:operation POST /networks/create compat compatCreateNetwork
	// ---
	// tags:
	//  - networks (compat)
	// summary: Create network
	// description: Create a network configuration
	// produces:
	// - application/json
	// parameters:
	//  - in: body
	//    name: create
	//    description: attributes for creating a container
	//    schema:
	//      $ref: "#/definitions/NetworkCreateRequest"
	// responses:
	//   200:
	//     $ref: "#/responses/CompatNetworkCreate"
	//   400:
	//     $ref: "#/responses/BadParamError"
	//   500:
	//     $ref: "#/responses/InternalError"
	r.HandleFunc(VersionedPath("/networks/create"), s.APIHandler(compat.CreateNetwork)).Methods(http.MethodPost)
	r.HandleFunc("/networks/create", s.APIHandler(compat.CreateNetwork)).Methods(http.MethodPost)
	// swagger:operation POST /networks/{name}/connect compat compatConnectNetwork
	// ---
	// tags:
	//  - networks (compat)
	// summary: Connect container to network
	// description: Connect a container to a network.  This endpoint is current a no-op
	// produces:
	// - application/json
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name of the network
	//  - in: body
	//    name: create
	//    description: attributes for connecting a container to a network
	//    schema:
	//      $ref: "#/definitions/NetworkConnectRequest"
	// responses:
	//   200:
	//     description: OK
	//   400:
	//     $ref: "#/responses/BadParamError"
	//   500:
	//     $ref: "#/responses/InternalError"
	r.HandleFunc(VersionedPath("/networks/{name}/connect"), s.APIHandler(compat.Connect)).Methods(http.MethodPost)
	r.HandleFunc("/networks/{name}/connect", s.APIHandler(compat.Connect)).Methods(http.MethodPost)
	// swagger:operation POST /networks/{name}/disconnect compat compatDisconnectNetwork
	// ---
	// tags:
	//  - networks (compat)
	// summary: Disconnect container from network
	// description: Disconnect a container from a network.  This endpoint is current a no-op
	// produces:
	// - application/json
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name of the network
	//  - in: body
	//    name: create
	//    description: attributes for disconnecting a container from a network
	//    schema:
	//      $ref: "#/definitions/NetworkDisconnectRequest"
	// responses:
	//   200:
	//     description: OK
	//   400:
	//     $ref: "#/responses/BadParamError"
	//   500:
	//     $ref: "#/responses/InternalError"
	r.HandleFunc(VersionedPath("/networks/{name}/disconnect"), s.APIHandler(compat.Disconnect)).Methods(http.MethodPost)
	r.HandleFunc("/networks/{name}/disconnect", s.APIHandler(compat.Disconnect)).Methods(http.MethodPost)

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
	//    name: force
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

	/*
		Libpod
	*/

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
	// parameters:
	//  - in: query
	//    name: filter
	//    type: string
	//    description: Provide filter values (e.g. 'name=podman')
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
