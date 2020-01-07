package server

import (
	"net/http"

	"github.com/containers/libpod/pkg/api/handlers"
	"github.com/containers/libpod/pkg/api/handlers/generic"
	"github.com/containers/libpod/pkg/api/handlers/libpod"
	"github.com/gorilla/mux"
)

func (s *APIServer) RegisterContainersHandlers(r *mux.Router) error {
	// swagger:operation POST /containers/create containers createContainer
	//
	// Create a container
	//
	// ---
	// produces:
	// - application/json
	// parameters:
	//  - in: query
	//    name: name
	//    type: string
	//    description: container name
	// responses:
	//   '201':
	//     schema:
	//     items:
	//       "$ref": "#/ctrCreateResponse"
	//   '400':
	//     description: bad parameter
	//     schema:
	//       "$ref": "#/types/ErrorModel"
	//   '404':
	//     description: no such container
	//     schema:
	//       "$ref": "#/types/ErrorModel"
	//   '409':
	//     description: conflict
	//     schema:
	//       "$ref": "#/types/ErrorModel"
	//   '500':
	//     description: unexpected error
	//     schema:
	//      "$ref": "#/types/ErrorModel"
	r.HandleFunc(VersionedPath("/containers/create"), APIHandler(s.Context, generic.CreateContainer)).Methods(http.MethodPost)
	// swagger:operation GET /containers/json containers listContainers
	//
	// List containers
	//
	// ---
	// produces:
	// - application/json
	// responses:
	//   '200':
	//     schema:
	//     type: array
	//     items:
	//       "$ref": "#/types/Container"
	//   '400':
	//     description: bad parameter
	//     schema:
	//       "$ref": "#/types/ErrorModel"
	//   '500':
	//     description: unexpected error
	//     schema:
	//      "$ref": "#/types/ErrorModel"
	r.HandleFunc(VersionedPath("/containers/json"), APIHandler(s.Context, generic.ListContainers)).Methods(http.MethodGet)
	// swagger:operation POST  /containers/prune containers pruneContainers
	//
	// Prune unused containers
	//
	// ---
	// parameters:
	//  - in: query
	//    name: filters
	//    type: map[string][]string
	//    description: something
	// produces:
	// - application/json
	// responses:
	//   '200':
	//     schema:
	//       "$ref": "#/types/ContainerPruneReport"
	//   '500':
	//     description: unexpected error
	//     schema:
	//      "$ref": "#/types/ErrorModel"
	r.HandleFunc(VersionedPath("/containers/prune"), APIHandler(s.Context, generic.PruneContainers)).Methods(http.MethodPost)
	// swagger:operation DELETE /containers/{nameOrID} containers removeContainer
	//
	// Delete container
	//
	// ---
	// parameters:
	//  - in: path
	//    name: nameOrID
	//    required: true
	//    description: the name or ID of the container
	//  - in: query
	//    name: force
	//    type: bool
	//    description: need something
	//  - in: query
	//    name: v
	//    type: bool
	//    description: need something
	//  - in: query
	//    name: link
	//    type: bool
	//    description: not supported
	// produces:
	// - application/json
	// responses:
	//   '200':
	//     schema:
	//     type: array
	//     items:
	//       "$ref": "#/types/Container"
	//   '400':
	//     description: bad parameter
	//     schema:
	//       "$ref": "#/types/ErrorModel"
	//   '404':
	//     description: no such container
	//     schema:
	//       "$ref": "#/types/ErrorModel"
	//   '409':
	//     description: conflict
	//     schema:
	//       "$ref": "#/types/ErrorModel"
	//   '500':
	//     description: unexpected error
	//     schema:
	//      "$ref": "#/types/ErrorModel"
	r.HandleFunc(VersionedPath("/containers/{name:..*}"), APIHandler(s.Context, generic.RemoveContainer)).Methods(http.MethodDelete)
	// swagger:operation GET /containers/{nameOrID}/json containers getContainer
	//
	// Inspect Container
	//
	// ---
	// parameters:
	//  - in: path
	//    name: nameOrID
	//    required: true
	//    description: the name or ID of the container
	// produces:
	// - application/json
	// responses:
	//   '200':
	//     schema:
	//     items:
	//       "$ref": "#/types/ContainerJSON"
	//   '404':
	//     description: no such container
	//     schema:
	//       "$ref": "#/types/ErrorModel"
	//   '500':
	//     description: unexpected error
	//     schema:
	//      "$ref": "#/types/ErrorModel"
	r.HandleFunc(VersionedPath("/containers/{name:..*}/json"), APIHandler(s.Context, generic.GetContainer)).Methods(http.MethodGet)
	// swagger:operation POST /containers/{nameOrID}/kill containers killContainer
	//
	// Kill Container
	//
	// ---
	// parameters:
	//  - in: path
	//    name: nameOrID
	//    required: true
	//    description: the name or ID of the container
	//  - in: query
	//    name: signal
	//    type: int
	//    description: signal to be sent to container
	// produces:
	// - application/json
	// responses:
	//   '204':
	//     description: no error
	//     schema:
	//   '404':
	//     description: no such container
	//     schema:
	//       "$ref": "#/types/ErrorModel"
	//   '409':
	//     description: conflict
	//     schema:
	//       "$ref": "#/types/ErrorModel"
	//   '500':
	//     description: unexpected error
	//     schema:
	//      "$ref": "#/types/ErrorModel"
	r.HandleFunc(VersionedPath("/containers/{name:..*}/kill"), APIHandler(s.Context, generic.KillContainer)).Methods(http.MethodPost)
	// swagger:operation GET /containers/{nameOrID}/logs containers LogsFromContainer
	//
	// Get logs from container
	//
	// ---
	// parameters:
	//  - in: path
	//    name: nameOrID
	//    required: true
	//    description: the name or ID of the container
	//  - in: query
	//    name: follow
	//    type: bool
	//    description: needs description
	//  - in: query
	//    name: stdout
	//    type: bool
	//    description: needs description
	//  - in: query
	//    name: stderr
	//    type: bool
	//    description: needs description
	//  - in: query
	//    name: since
	//    type:  string
	//    description: needs description
	//  - in: query
	//    name: until
	//    type:  string
	//    description: needs description
	//  - in: query
	//    name: timestamps
	//    type: bool
	//    description: needs description
	//  - in: query
	//    name: tail
	//    type: string
	//    description: needs description
	// produces:
	// - application/json
	// responses:
	//   '200':
	//     description: no error
	//     schema:
	//   '404':
	//     description: no such container
	//     schema:
	//       "$ref": "#/types/ErrorModel"
	//   '500':
	//     description: unexpected error
	//     schema:
	//      "$ref": "#/types/ErrorModel"
	r.HandleFunc(VersionedPath("/containers/{name:..*}/logs"), APIHandler(s.Context, generic.LogsFromContainer)).Methods(http.MethodGet)
	// swagger:operation POST /containers/{nameOrID}/pause containers pauseContainer
	//
	// Pause Container
	//
	// ---
	// parameters:
	//  - in: path
	//    name: nameOrID
	//    required: true
	//    description: the name or ID of the container
	// produces:
	// - application/json
	// responses:
	//   '204':
	//     description: no error
	//     schema:
	//   '404':
	//     description: no such container
	//     schema:
	//       "$ref": "#/types/ErrorModel"
	//   '500':
	//     description: unexpected error
	//     schema:
	//      "$ref": "#/types/ErrorModel"
	r.HandleFunc(VersionedPath("/containers/{name:..*}/pause"), APIHandler(s.Context, handlers.PauseContainer)).Methods(http.MethodPost)
	r.HandleFunc(VersionedPath("/containers/{name:..*}/rename"), APIHandler(s.Context, handlers.UnsupportedHandler)).Methods(http.MethodPost)
	// swagger:operation POST /containers/{nameOrID}/restart containers restartContainer
	//
	// Restart Container
	//
	// ---
	// parameters:
	//  - in: path
	//    name: nameOrID
	//    required: true
	//    description: the name or ID of the container
	//  - in: query
	//    name: t
	//    type: int
	//    description: timeout before sending kill signal to container
	// produces:
	// - application/json
	// responses:
	//   '204':
	//     description: no error
	//     schema:
	//   '404':
	//     description: no such container
	//     schema:
	//       "$ref": "#/types/ErrorModel"
	//   '500':
	//     description: unexpected error
	//     schema:
	//      "$ref": "#/types/ErrorModel"
	r.HandleFunc(VersionedPath("/containers/{name:..*}/restart"), APIHandler(s.Context, handlers.RestartContainer)).Methods(http.MethodPost)
	// swagger:operation POST /containers/{nameOrID}/start containers startContainer
	//
	// Start a container
	//
	// ---
	// parameters:
	//  - in: path
	//    name: nameOrID
	//    required: true
	//    description: the name or ID of the container
	//  - in: query
	//    name: detachKeys
	//    type: string
	//    description: needs description
	// produces:
	// - application/json
	// responses:
	//   '204':
	//     description: no error
	//   '304':
	//     description: container already started
	//     schema:
	//       "$ref": "#/types/ErrorModel"
	//   '404':
	//     description: no such container
	//     schema:
	//       "$ref": "#/types/ErrorModel"
	//   '500':
	//     description: unexpected error
	//     schema:
	//      "$ref": "#/types/ErrorModel"
	r.HandleFunc(VersionedPath("/containers/{name:..*}/start"), APIHandler(s.Context, handlers.StartContainer)).Methods(http.MethodPost)
	// swagger:operation GET /containers/{nameOrID}/stats containers statsContainer
	//
	//  Get stats for a contrainer
	//
	// ---
	// parameters:
	//  - in: path
	//    name: nameOrID
	//    required: true
	//    description: the name or ID of the container
	//  - in: query
	//    name: stream
	//    type: bool
	//    description: needs description
	// produces:
	// - application/json
	// responses:
	//   '200':
	//     description: no error
	//     schema:
	//       "ref": "#/handler/stats"
	//   '304':
	//     description: container already started
	//     schema:
	//       "$ref": "#/types/ErrorModel"
	//   '404':
	//     description: no such container
	//     schema:
	//       "$ref": "#/types/ErrorModel"
	//   '500':
	//     description: unexpected error
	//     schema:
	//      "$ref": "#/types/ErrorModel"
	r.HandleFunc(VersionedPath("/containers/{name:..*}/stats"), APIHandler(s.Context, generic.StatsContainer)).Methods(http.MethodGet)
	// swagger:operation POST /containers/{nameOrID}/stop containers stopContainer
	//
	// Stop a container
	//
	// ---
	// parameters:
	//  - in: path
	//    name: nameOrID
	//    required: true
	//    description: the name or ID of the container
	//  - in: query
	//    name: t
	//    type: int
	//    description: number of seconds to wait before killing container
	// produces:
	// - application/json
	// responses:
	//   '204':
	//     description: no error
	//   '304':
	//     description: container already stopped
	//     schema:
	//       "$ref": "#/types/ErrorModel"
	//   '404':
	//     description: no such container
	//     schema:
	//       "$ref": "#/types/ErrorModel"
	//   '500':
	//     description: unexpected error
	//     schema:
	//      "$ref": "#/types/ErrorModel"
	r.HandleFunc(VersionedPath("/containers/{name:..*}/stop"), APIHandler(s.Context, handlers.StopContainer)).Methods(http.MethodPost)
	// swagger:operation GET /containers/{nameOrID}/top containers topContainer
	//
	// List processes running inside a container
	//
	// ---
	// parameters:
	//  - in: path
	//    name: nameOrID
	//    required: true
	//    description: the name or ID of the container
	//  - in: query
	//    name: ps_args
	//    type: string
	//    description: arguments to pass to ps such as aux
	// produces:
	// - application/json
	// responses:
	//   '200':
	//     description: no error
	//     schema:
	//       "ref": "#/types/ContainerTopBody"
	//   '404':
	//     description: no such container
	//     schema:
	//       "$ref": "#/types/ErrorModel"
	//   '500':
	//     description: unexpected error
	//     schema:
	//      "$ref": "#/types/ErrorModel"
	r.HandleFunc(VersionedPath("/containers/{name:..*}/top"), APIHandler(s.Context, handlers.TopContainer)).Methods(http.MethodGet)
	// swagger:operation POST /containers/{nameOrID}/unpause containers unpauseContainer
	//
	// Unpause Container
	//
	// ---
	// parameters:
	//  - in: path
	//    name: nameOrID
	//    required: true
	//    description: the name or ID of the container
	// produces:
	// - application/json
	// responses:
	//   '204':
	//     description: no error
	//     schema:
	//   '404':
	//     description: no such container
	//     schema:
	//       "$ref": "#/types/ErrorModel"
	//   '500':
	//     description: unexpected error
	//     schema:
	//      "$ref": "#/types/ErrorModel"
	r.HandleFunc(VersionedPath("/containers/{name:..*}/unpause"), APIHandler(s.Context, handlers.UnpauseContainer)).Methods(http.MethodPost)
	// swagger:operation POST /containers/{nameOrID}/wait containers waitContainer
	//
	// Wait on a container to exit
	//
	// ---
	// parameters:
	//  - in: path
	//    name: nameOrID
	//    required: true
	//    description: the name or ID of the container
	//  - in: query
	//    name: condition
	//    type: string
	//    description: Wait until the container reaches the given condition
	// produces:
	// - application/json
	// responses:
	//   '204':
	//     description: no error
	//     schema:
	//   '404':
	//     description: no such container
	//     schema:
	//       "$ref": "#/types/ErrorModel"
	//   '500':
	//     description: unexpected error
	//     schema:
	//      "$ref": "#/types/ErrorModel"
	r.HandleFunc(VersionedPath("/containers/{name:..*}/wait"), APIHandler(s.Context, generic.WaitContainer)).Methods(http.MethodPost)

	/*
		libpod endpoints
	*/

	r.HandleFunc(VersionedPath("/libpod/containers/create"), APIHandler(s.Context, libpod.CreateContainer)).Methods(http.MethodPost)
	// swagger:operation GET /libpod/containers/json containers listContainers
	//
	// List containers
	//
	// ---
	// produces:
	// - application/json
	// responses:
	//   '200':
	//     schema:
	//     type: array
	//     items:
	//       "$ref": "#/shared/GetPsContainerOutput"
	//   '400':
	//     description: bad parameter
	//     schema:
	//       "$ref": "#/types/ErrorModel"
	//   '500':
	//     description: unexpected error
	//     schema:
	//      "$ref": "#/types/ErrorModel"
	r.HandleFunc(VersionedPath("/libpod/containers/json"), APIHandler(s.Context, libpod.ListContainers)).Methods(http.MethodGet)
	// swagger:operation POST /libpod/containers/prune containers pruneContainers
	//
	// Prune unused containers
	//
	// ---
	// parameters:
	//  - in: query
	//    name: force
	//    type: bool
	//    description: something
	//  - in: query
	//    name: filters
	//    type: map[string][]string
	//    description: something
	// produces:
	// - application/json
	// responses:
	//   '200':
	//     schema:
	//       "$ref": "#/types/ContainerPruneReport"
	//   '500':
	//     description: unexpected error
	//     schema:
	//      "$ref": "#/types/ErrorModel"
	r.HandleFunc(VersionedPath("/libpod/containers/prune"), APIHandler(s.Context, libpod.PruneContainers)).Methods(http.MethodPost)
	// swagger:operation GET /libpod/containers/showmounted containers showMounterContainers
	//
	// Show mounted containers
	//
	// ---
	// produces:
	// - application/json
	// responses:
	//   '200':
	//     schema:
	//       "$ref": "TBD"
	//   '500':
	//     description: unexpected error
	//     schema:
	//      "$ref": "#/types/ErrorModel"
	r.HandleFunc(VersionedPath("/libpod/containers/showmounted"), APIHandler(s.Context, libpod.ShowMountedContainers)).Methods(http.MethodGet)
	// swagger:operation DELETE /libpod/containers/json containers removeContainer
	//
	// Delete container
	//
	// ---
	// parameters:
	//  - in: path
	//    name: nameOrID
	//    required: true
	//    description: the name or ID of the container
	//  - in: query
	//    name: force
	//    type: bool
	//    description: need something
	//  - in: query
	//    name: v
	//    type: bool
	//    description: need something
	// produces:
	// - application/json
	// responses:
	//   '200':
	//     schema:
	//     type: array
	//     items:
	//       "$ref": "#/types/Container"
	//   '400':
	//     description: bad parameter
	//     schema:
	//       "$ref": "#/types/ErrorModel"
	//   '404':
	//     description: no such container
	//     schema:
	//       "$ref": "#/types/ErrorModel"
	//   '409':
	//     description: conflict
	//     schema:
	//       "$ref": "#/types/ErrorModel"
	//   '500':
	//     description: unexpected error
	//     schema:
	//      "$ref": "#/types/ErrorModel"
	r.HandleFunc(VersionedPath("/libpod/containers/{name:..*}"), APIHandler(s.Context, libpod.RemoveContainer)).Methods(http.MethodDelete)
	// swagger:operation GET /libpod/containers/{nameOrID}/json containers getContainer
	//
	// Inspect Container
	//
	// ---
	// parameters:
	//  - in: path
	//    name: nameOrID
	//    required: true
	//    description: the name or ID of the container
	//  - in: query
	//    name: size
	//    type: bool
	//    description: display filesystem usage
	// produces:
	// - application/json
	// responses:
	//   '200':
	//     schema:
	//     items:
	//       "$ref": "#InspectContainerData"
	//   '404':
	//     description: no such container
	//     schema:
	//       "$ref": "#/types/ErrorModel"
	//   '500':
	//     description: unexpected error
	//     schema:
	//      "$ref": "#/types/ErrorModel"
	r.HandleFunc(VersionedPath("/libpod/containers/{name:..*}/json"), APIHandler(s.Context, libpod.GetContainer)).Methods(http.MethodGet)
	// swagger:operation POST /libpod/containers/{nameOrID}/kill containers killContainer
	//
	// Kill Container
	//
	// ---
	// parameters:
	//  - in: path
	//    name: nameOrID
	//    required: true
	//    description: the name or ID of the container
	//  - in: query
	//    name: signal
	//    type: int
	//    default: 15
	//    description: signal to be sent to container
	// produces:
	// - application/json
	// responses:
	//   '204':
	//     description: no error
	//     schema:
	//   '404':
	//     description: no such container
	//     schema:
	//       "$ref": "#/types/ErrorModel"
	//   '409':
	//     description: conflict
	//     schema:
	//       "$ref": "#/types/ErrorModel"
	//   '500':
	//     description: unexpected error
	//     schema:
	//      "$ref": "#/types/ErrorModel"
	r.HandleFunc(VersionedPath("/libpod/containers/{name:..*}/kill"), APIHandler(s.Context, libpod.KillContainer)).Methods(http.MethodGet)
	// swagger:operation GET /libpod/containers/{nameOrID}/mount containers mountContainer
	//
	// Mount a container
	//
	// ---
	// parameters:
	//  - in: path
	//    name: nameOrID
	//    required: true
	//    description: the name or ID of the container
	// produces:
	// - application/json
	// responses:
	//   '200':
	//     schema:
	//     items:
	//       "$ref": "string"
	//   '404':
	//     description: no such container
	//     schema:
	//       "$ref": "#/types/ErrorModel"
	//   '500':
	//     description: unexpected error
	//     schema:
	//      "$ref": "#/types/ErrorModel"
	r.HandleFunc(VersionedPath("/libpod/containers/{name:..*}/mount"), APIHandler(s.Context, libpod.LogsFromContainer)).Methods(http.MethodPost)
	r.HandleFunc(VersionedPath("/libpod/containers/{name:..*}/logs"), APIHandler(s.Context, libpod.LogsFromContainer)).Methods(http.MethodGet)
	// swagger:operation POST /libpod/containers/{nameOrID}/pause containers pauseContainer
	//
	// Pause Container
	//
	// ---
	// parameters:
	//  - in: path
	//    name: nameOrID
	//    required: true
	//    description: the name or ID of the container
	// produces:
	// - application/json
	// responses:
	//   '204':
	//     description: no error
	//     schema:
	//   '404':
	//     description: no such container
	//     schema:
	//       "$ref": "#/types/ErrorModel"
	//   '500':
	//     description: unexpected error
	//     schema:
	//      "$ref": "#/types/ErrorModel"
	r.HandleFunc(VersionedPath("/libpod/containers/{name:..*}/pause"), APIHandler(s.Context, handlers.PauseContainer)).Methods(http.MethodPost)
	// swagger:operation POST /libpod/containers/{nameOrID}/restart containers restartContainer
	//
	// Restart Container
	//
	// ---
	// parameters:
	//  - in: path
	//    name: nameOrID
	//    required: true
	//    description: the name or ID of the container
	//  - in: query
	//    name: t
	//    type: int
	//    description: timeout before sending kill signal to container
	// produces:
	// - application/json
	// responses:
	//   '204':
	//     description: no error
	//     schema:
	//   '404':
	//     description: no such container
	//     schema:
	//       "$ref": "#/types/ErrorModel"
	//   '500':
	//     description: unexpected error
	//     schema:
	//      "$ref": "#/types/ErrorModel"
	r.HandleFunc(VersionedPath("/libpod/containers/{name:..*}/restart"), APIHandler(s.Context, handlers.RestartContainer)).Methods(http.MethodPost)
	// swagger:operation POST /libpod/containers/{nameOrID}/start containers startContainer
	//
	// Start a container
	//
	// ---
	// parameters:
	//  - in: path
	//    name: nameOrID
	//    required: true
	//    description: the name or ID of the container
	//  - in: query
	//    name: detachKeys
	//    type: string
	//    description: needs description
	// produces:
	// - application/json
	// responses:
	//   '204':
	//     description: no error
	//   '304':
	//     description: container already started
	//     schema:
	//       "$ref": "#/types/ErrorModel"
	//   '404':
	//     description: no such container
	//     schema:
	//       "$ref": "#/types/ErrorModel"
	//   '500':
	//     description: unexpected error
	//     schema:
	//      "$ref": "#/types/ErrorModel"
	r.HandleFunc(VersionedPath("/libpod/containers/{name:..*}/start"), APIHandler(s.Context, handlers.StartContainer)).Methods(http.MethodPost)
	r.HandleFunc(VersionedPath("/libpod/containers/{name:..*}/stats"), APIHandler(s.Context, libpod.StatsContainer)).Methods(http.MethodGet)
	r.HandleFunc(VersionedPath("/libpod/containers/{name:..*}/top"), APIHandler(s.Context, handlers.TopContainer)).Methods(http.MethodGet)
	// swagger:operation POST /libpod/containers/{nameOrID}/unpause containers unpauseContainer
	//
	// Unpause Container
	//
	// ---
	// parameters:
	//  - in: path
	//    name: nameOrID
	//    required: true
	//    description: the name or ID of the container
	// produces:
	// - application/json
	// responses:
	//   '204':
	//     description: no error
	//     schema:
	//   '404':
	//     description: no such container
	//     schema:
	//       "$ref": "#/types/ErrorModel"
	//   '500':
	//     description: unexpected error
	//     schema:
	//      "$ref": "#/types/ErrorModel"
	r.HandleFunc(VersionedPath("/libpod/containers/{name:..*}/unpause"), APIHandler(s.Context, handlers.UnpauseContainer)).Methods(http.MethodPost)
	// swagger:operation POST /libpod/containers/{nameOrID}/wait containers waitContainer
	//
	// Wait on a container to exit
	//
	// ---
	// parameters:
	//  - in: path
	//    name: nameOrID
	//    required: true
	//    description: the name or ID of the container
	//  - in: query
	//    name: condition
	//    type: string
	//    description: Wait until the container reaches the given condition
	// produces:
	// - application/json
	// responses:
	//   '204':
	//     description: no error
	//     schema:
	//   '404':
	//     description: no such container
	//     schema:
	//       "$ref": "#/types/ErrorModel"
	//   '500':
	//     description: unexpected error
	//     schema:
	//      "$ref": "#/types/ErrorModel"
	r.HandleFunc(VersionedPath("/libpod/containers/{name:..*}/wait"), APIHandler(s.Context, libpod.WaitContainer)).Methods(http.MethodPost)
	// swagger:operation POST /libpod/containers/{nameOrID}/exists containers containerExists
	//
	// Check if container exists
	//
	// ---
	// parameters:
	//  - in: path
	//    name: nameOrID
	//    required: true
	//    description: the name or ID of the container
	// produces:
	// - application/json
	// responses:
	//   '204':
	//     description: no error
	//     schema:
	//   '404':
	//     description: no such container
	//     schema:
	//       "$ref": "#/types/ErrorModel"
	//   '500':
	//     description: unexpected error
	//     schema:
	//      "$ref": "#/types/ErrorModel"
	r.HandleFunc(VersionedPath("/libpod/containers/{name:..*}/exists"), APIHandler(s.Context, libpod.ContainerExists)).Methods(http.MethodGet)
	// swagger:operation POST /libpod/containers/{nameOrID}/stop containers stopContainer
	//
	// Stop a container
	//
	// ---
	// parameters:
	//  - in: path
	//    name: nameOrID
	//    required: true
	//    description: the name or ID of the container
	//  - in: query
	//    name: t
	//    type: int
	//    description: number of seconds to wait before killing container
	// produces:
	// - application/json
	// responses:
	//   '204':
	//     description: no error
	//   '304':
	//     description: container already stopped
	//     schema:
	//       "$ref": "#/types/ErrorModel"
	//   '404':
	//     description: no such container
	//     schema:
	//       "$ref": "#/types/ErrorModel"
	//   '500':
	//     description: unexpected error
	//     schema:
	//      "$ref": "#/types/ErrorModel"
	r.HandleFunc(VersionedPath("/libpod/containers/{name:..*}/stop"), APIHandler(s.Context, handlers.StopContainer)).Methods(http.MethodPost)
	return nil
}
