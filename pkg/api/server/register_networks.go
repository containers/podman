package server

import (
	"net/http"

	"github.com/containers/podman/v4/pkg/api/handlers/compat"
	"github.com/containers/podman/v4/pkg/api/handlers/libpod"
	"github.com/gorilla/mux"
)

func (s *APIServer) registerNetworkHandlers(r *mux.Router) error {
	// swagger:operation DELETE /networks/{name} compat NetworkDelete
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
	// swagger:operation GET /networks/{name} compat NetworkInspect
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
	//  - in: query
	//    name: verbose
	//    type: boolean
	//    required: false
	//    description: Detailed inspect output for troubleshooting
	//  - in: query
	//    name: scope
	//    type: string
	//    required: false
	//    description: Filter the network by scope (swarm, global, or local)
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
	// swagger:operation GET /networks compat NetworkList
	// ---
	// tags:
	//  - networks (compat)
	// summary: List networks
	// description: Display summary of network configurations
	// parameters:
	//  - in: query
	//    name: filters
	//    type: string
	//    description: |
	//      JSON encoded value of the filters (a `map[string][]string`) to process on the network list. Currently available filters:
	//        - `name=[name]` Matches network name (accepts regex).
	//        - `id=[id]` Matches for full or partial ID.
	//        - `driver=[driver]` Only bridge is supported.
	//        - `label=[key]` or `label=[key=value]` Matches networks based on the presence of a label alone or a label and a value.
	// produces:
	// - application/json
	// responses:
	//   200:
	//     $ref: "#/responses/CompatNetworkList"
	//   500:
	//     $ref: "#/responses/InternalError"
	r.HandleFunc(VersionedPath("/networks"), s.APIHandler(compat.ListNetworks)).Methods(http.MethodGet)
	r.HandleFunc("/networks", s.APIHandler(compat.ListNetworks)).Methods(http.MethodGet)
	// swagger:operation POST /networks/create compat NetworkCreate
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
	//    description: attributes for creating a network
	//    schema:
	//      $ref: "#/definitions/NetworkCreateRequest"
	// responses:
	//   201:
	//     description: network created
	//     schema:
	//       type: object
	//       properties:
	//         Id:
	//           type: string
	//         Warning:
	//           type: string
	//   400:
	//     $ref: "#/responses/BadParamError"
	//   500:
	//     $ref: "#/responses/InternalError"
	r.HandleFunc(VersionedPath("/networks/create"), s.APIHandler(compat.CreateNetwork)).Methods(http.MethodPost)
	r.HandleFunc("/networks/create", s.APIHandler(compat.CreateNetwork)).Methods(http.MethodPost)
	// swagger:operation POST /networks/{name}/connect compat NetworkConnect
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
	//      $ref: "#/definitions/NetworkCompatConnectRequest"
	// responses:
	//   200:
	//     description: OK
	//   400:
	//     $ref: "#/responses/BadParamError"
	//   500:
	//     $ref: "#/responses/InternalError"
	r.HandleFunc(VersionedPath("/networks/{name}/connect"), s.APIHandler(compat.Connect)).Methods(http.MethodPost)
	r.HandleFunc("/networks/{name}/connect", s.APIHandler(compat.Connect)).Methods(http.MethodPost)
	// swagger:operation POST /networks/{name}/disconnect compat NetworkDisconnect
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
	//      $ref: "#/definitions/NetworkCompatDisconnectRequest"
	// responses:
	//   200:
	//     description: OK
	//   400:
	//     $ref: "#/responses/BadParamError"
	//   500:
	//     $ref: "#/responses/InternalError"
	r.HandleFunc(VersionedPath("/networks/{name}/disconnect"), s.APIHandler(compat.Disconnect)).Methods(http.MethodPost)
	r.HandleFunc("/networks/{name}/disconnect", s.APIHandler(compat.Disconnect)).Methods(http.MethodPost)
	// swagger:operation POST /networks/prune compat NetworkPrune
	// ---
	// tags:
	//  - networks (compat)
	// summary: Delete unused networks
	// description: Remove CNI networks that do not have containers
	// produces:
	// - application/json
	// parameters:
	//  - in: query
	//    name: filters
	//    type: string
	//    description: |
	//      Filters to process on the prune list, encoded as JSON (a map[string][]string).
	//      Available filters:
	//        - `until=<timestamp>` Prune networks created before this timestamp. The <timestamp> can be Unix timestamps, date formatted timestamps, or Go duration strings (e.g. `10m`, `1h30m`) computed relative to the daemon machine’s time.
	//        - `label` (`label=<key>`, `label=<key>=<value>`, `label!=<key>`, or `label!=<key>=<value>`) Prune networks with (or without, in case `label!=...` is used) the specified labels.
	// responses:
	//   200:
	//     description: OK
	//     schema:
	//       type: object
	//       properties:
	//         NetworksDeleted:
	//           type: array
	//           items:
	//             type: string
	//   500:
	//     $ref: "#/responses/InternalError"
	r.HandleFunc(VersionedPath("/networks/prune"), s.APIHandler(compat.Prune)).Methods(http.MethodPost)
	r.HandleFunc("/networks/prune", s.APIHandler(compat.Prune)).Methods(http.MethodPost)

	// swagger:operation DELETE /libpod/networks/{name} libpod NetworkDeleteLibpod
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
	r.HandleFunc(VersionedPath("/libpod/networks/{name}"), s.APIHandler(libpod.RemoveNetwork)).Methods(http.MethodDelete)
	// swagger:operation GET /libpod/networks/{name}/exists libpod NetworkExistsLibpod
	// ---
	// tags:
	//  - networks
	// summary: Network exists
	// description: Check if network exists
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the network
	// produces:
	// - application/json
	// responses:
	//   204:
	//     description: network exists
	//   404:
	//     $ref: '#/responses/NoSuchNetwork'
	//   500:
	//     $ref: '#/responses/InternalError'
	r.Handle(VersionedPath("/libpod/networks/{name}/exists"), s.APIHandler(libpod.ExistsNetwork)).Methods(http.MethodGet)
	// swagger:operation GET /libpod/networks/json libpod NetworkListLibpod
	// ---
	// tags:
	//  - networks
	// summary: List networks
	// description: |
	//   Display summary of network configurations.
	//     - In a 200 response, all of the fields named Bytes are returned as a Base64 encoded string.
	// parameters:
	//  - in: query
	//    name: filters
	//    type: string
	//    description: |
	//      JSON encoded value of the filters (a `map[string][]string`) to process on the network list. Available filters:
	//        - `name=[name]` Matches network name (accepts regex).
	//        - `id=[id]` Matches for full or partial ID.
	//        - `driver=[driver]` Only bridge is supported.
	//        - `label=[key]` or `label=[key=value]` Matches networks based on the presence of a label alone or a label and a value.
	//        - `until=[timestamp]` Matches all networks that were create before the given timestamp.
	// produces:
	// - application/json
	// responses:
	//   200:
	//     $ref: "#/responses/NetworkListReport"
	//   500:
	//     $ref: "#/responses/InternalError"
	r.HandleFunc(VersionedPath("/libpod/networks/json"), s.APIHandler(libpod.ListNetworks)).Methods(http.MethodGet)
	// swagger:operation GET /libpod/networks/{name}/json libpod NetworkInspectLibpod
	// ---
	// tags:
	//  - networks
	// summary: Inspect a network
	// description: |
	//   Display low level configuration for a CNI network.
	//     - In a 200 response, all of the fields named Bytes are returned as a Base64 encoded string.
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
	r.HandleFunc(VersionedPath("/libpod/networks/{name}"), s.APIHandler(libpod.InspectNetwork)).Methods(http.MethodGet)
	// swagger:operation POST /libpod/networks/create libpod NetworkCreateLibpod
	// ---
	// tags:
	//  - networks
	// summary: Create network
	// description: Create a new network configuration
	// produces:
	// - application/json
	// parameters:
	//  - in: body
	//    name: create
	//    description: attributes for creating a network
	//    schema:
	//      $ref: "#/definitions/NetworkCreateLibpod"
	// responses:
	//   200:
	//     $ref: "#/responses/NetworkCreateReport"
	//   400:
	//     $ref: "#/responses/BadParamError"
	//   409:
	//     $ref: "#/responses/ConflictError"
	//   500:
	//     $ref: "#/responses/InternalError"
	r.HandleFunc(VersionedPath("/libpod/networks/create"), s.APIHandler(libpod.CreateNetwork)).Methods(http.MethodPost)
	// swagger:operation POST /libpod/networks/{name}/connect libpod NetworkConnectLibpod
	// ---
	// tags:
	//  - networks
	// summary: Connect container to network
	// description: Connect a container to a network.
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
	//   404:
	//     $ref: "#/responses/NoSuchNetwork"
	//   500:
	//     $ref: "#/responses/InternalError"
	r.HandleFunc(VersionedPath("/libpod/networks/{name}/connect"), s.APIHandler(libpod.Connect)).Methods(http.MethodPost)
	// swagger:operation POST /libpod/networks/{name}/disconnect libpod NetworkDisconnectLibpod
	// ---
	// tags:
	//  - networks
	// summary: Disconnect container from network
	// description: Disconnect a container from a network.
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
	//      $ref: "#/definitions/NetworkCompatDisconnectRequest"
	// responses:
	//   200:
	//     description: OK
	//   404:
	//     $ref: "#/responses/NoSuchNetwork"
	//   500:
	//     $ref: "#/responses/InternalError"
	r.HandleFunc(VersionedPath("/libpod/networks/{name}/disconnect"), s.APIHandler(compat.Disconnect)).Methods(http.MethodPost)
	// swagger:operation POST /libpod/networks/prune libpod NetworkPruneLibpod
	// ---
	// tags:
	//  - networks
	// summary: Delete unused networks
	// description: Remove CNI networks that do not have containers
	// produces:
	// - application/json
	// parameters:
	//  - in: query
	//    name: filters
	//    type: string
	//    description: |
	//      Filters to process on the prune list, encoded as JSON (a `map[string][]string`).
	//      Available filters:
	//        - `until=<timestamp>` Prune networks created before this timestamp. The `<timestamp>` can be Unix timestamps, date formatted timestamps, or Go duration strings (e.g. `10m`, `1h30m`) computed relative to the daemon machine’s time.
	//        - `label` (`label=<key>`, `label=<key>=<value>`, `label!=<key>`, or `label!=<key>=<value>`) Prune networks with (or without, in case `label!=...` is used) the specified labels.
	// responses:
	//   200:
	//     $ref: "#/responses/NetworkPruneResponse"
	//   500:
	//     $ref: "#/responses/InternalError"
	r.HandleFunc(VersionedPath("/libpod/networks/prune"), s.APIHandler(libpod.Prune)).Methods(http.MethodPost)
	return nil
}
