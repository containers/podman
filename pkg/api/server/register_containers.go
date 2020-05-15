package server

import (
	"net/http"

	"github.com/containers/libpod/pkg/api/handlers/compat"
	"github.com/containers/libpod/pkg/api/handlers/libpod"
	"github.com/gorilla/mux"
)

func (s *APIServer) registerContainersHandlers(r *mux.Router) error {
	// swagger:operation POST /containers/create compat createContainer
	// ---
	//   summary: Create a container
	//   tags:
	//    - containers (compat)
	//   produces:
	//   - application/json
	//   parameters:
	//    - in: query
	//      name: name
	//      type: string
	//      description: container name
	//   responses:
	//     201:
	//       $ref: "#/responses/ContainerCreateResponse"
	//     400:
	//       $ref: "#/responses/BadParamError"
	//     404:
	//       $ref: "#/responses/NoSuchContainer"
	//     409:
	//       $ref: "#/responses/ConflictError"
	//     500:
	//       $ref: "#/responses/InternalError"
	r.HandleFunc(VersionedPath("/containers/create"), s.APIHandler(compat.CreateContainer)).Methods(http.MethodPost)
	// Added non version path to URI to support docker non versioned paths
	r.HandleFunc("/containers/create", s.APIHandler(compat.CreateContainer)).Methods(http.MethodPost)
	// swagger:operation GET /containers/json compat listContainers
	// ---
	// tags:
	//  - containers (compat)
	// summary: List containers
	// description: Returns a list of containers
	// parameters:
	//  - in: query
	//    name: all
	//    type: boolean
	//    default: false
	//    description: Return all containers. By default, only running containers are shown
	//  - in: query
	//    name: limit
	//    description: Return this number of most recently created containers, including non-running ones.
	//    type: integer
	//  - in: query
	//    name: size
	//    type: boolean
	//    default: false
	//    description: Return the size of container as fields SizeRw and SizeRootFs.
	//  - in: query
	//    name: filters
	//    type: string
	//    description: |
	//       Returns a list of containers.
	//        - ancestor=(<image-name>[:<tag>], <image id>, or <image@digest>)
	//        - before=(<container id> or <container name>)
	//        - expose=(<port>[/<proto>]|<startport-endport>/[<proto>])
	//        - exited=<int> containers with exit code of <int>
	//        - health=(starting|healthy|unhealthy|none)
	//        - id=<ID> a container's ID
	//        - is-task=(true|false)
	//        - label=key or label="key=value" of a container label
	//        - name=<name> a container's name
	//        - network=(<network id> or <network name>)
	//        - publish=(<port>[/<proto>]|<startport-endport>/[<proto>])
	//        - since=(<container id> or <container name>)
	//        - status=(created|restarting|running|removing|paused|exited|dead)
	//        - volume=(<volume name> or <mount point destination>)
	// produces:
	// - application/json
	// responses:
	//   200:
	//     $ref: "#/responses/DocsListContainer"
	//   400:
	//     $ref: "#/responses/BadParamError"
	//   500:
	//     $ref: "#/responses/InternalError"
	r.HandleFunc(VersionedPath("/containers/json"), s.APIHandler(compat.ListContainers)).Methods(http.MethodGet)
	// Added non version path to URI to support docker non versioned paths
	r.HandleFunc("/containers/json", s.APIHandler(compat.ListContainers)).Methods(http.MethodGet)
	// swagger:operation POST  /containers/prune compat pruneContainers
	// ---
	// tags:
	//   - containers (compat)
	// summary: Delete stopped containers
	// description: Remove containers not in use
	// parameters:
	//  - in: query
	//    name: filters
	//    type: string
	//    description:  |
	//      Filters to process on the prune list, encoded as JSON (a `map[string][]string`).  Available filters:
	//       - `until=<timestamp>` Prune containers created before this timestamp. The `<timestamp>` can be Unix timestamps, date formatted timestamps, or Go duration strings (e.g. `10m`, `1h30m`) computed relative to the daemon machine’s time.
	//       - `label` (`label=<key>`, `label=<key>=<value>`, `label!=<key>`, or `label!=<key>=<value>`) Prune containers with (or without, in case `label!=...` is used) the specified labels.
	// produces:
	// - application/json
	// responses:
	//   200:
	//     $ref: "#/responses/DocsContainerPruneReport"
	//   500:
	//     $ref: "#/responses/InternalError"
	r.HandleFunc(VersionedPath("/containers/prune"), s.APIHandler(compat.PruneContainers)).Methods(http.MethodPost)
	// Added non version path to URI to support docker non versioned paths
	r.HandleFunc("/containers/prune", s.APIHandler(compat.PruneContainers)).Methods(http.MethodPost)
	// swagger:operation DELETE /containers/{name} compat removeContainer
	// ---
	// tags:
	//  - containers (compat)
	// summary: Remove a container
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	//  - in: query
	//    name: force
	//    type: boolean
	//    default: false
	//    description: If the container is running, kill it before removing it.
	//  - in: query
	//    name: v
	//    type: boolean
	//    default: false
	//    description: Remove the volumes associated with the container.
	//  - in: query
	//    name: link
	//    type: boolean
	//    description: not supported
	// produces:
	// - application/json
	// responses:
	//   204:
	//     description: no error
	//   400:
	//     $ref: "#/responses/BadParamError"
	//   404:
	//     $ref: "#/responses/NoSuchContainer"
	//   409:
	//     $ref: "#/responses/ConflictError"
	//   500:
	//     $ref: "#/responses/InternalError"
	r.HandleFunc(VersionedPath("/containers/{name}"), s.APIHandler(compat.RemoveContainer)).Methods(http.MethodDelete)
	// Added non version path to URI to support docker non versioned paths
	r.HandleFunc("/containers/{name}", s.APIHandler(compat.RemoveContainer)).Methods(http.MethodDelete)
	// swagger:operation GET /containers/{name}/json compat getContainer
	// ---
	// tags:
	//  - containers (compat)
	// summary: Inspect container
	// description: Return low-level information about a container.
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or id of the container
	//  - in: query
	//    name: size
	//    type: boolean
	//    default: false
	//    description: include the size of the container
	// produces:
	// - application/json
	// responses:
	//   200:
	//     $ref: "#/responses/DocsContainerInspectResponse"
	//   404:
	//     $ref: "#/responses/NoSuchContainer"
	//   500:
	//     $ref: "#/responses/InternalError"
	r.HandleFunc(VersionedPath("/containers/{name}/json"), s.APIHandler(compat.GetContainer)).Methods(http.MethodGet)
	// Added non version path to URI to support docker non versioned paths
	r.HandleFunc("/containers/{name}/json", s.APIHandler(compat.GetContainer)).Methods(http.MethodGet)
	// swagger:operation POST /containers/{name}/kill compat killContainer
	// ---
	// tags:
	//   - containers (compat)
	// summary: Kill container
	// description: Signal to send to the container as an integer or string (e.g. SIGINT)
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	//  - in: query
	//    name: signal
	//    type: string
	//    default: TERM
	//    description: signal to be sent to container
	//    default: SIGKILL
	// produces:
	// - application/json
	// responses:
	//   204:
	//     description: no error
	//   404:
	//     $ref: "#/responses/NoSuchContainer"
	//   409:
	//     $ref: "#/responses/ConflictError"
	//   500:
	//     $ref: "#/responses/InternalError"
	r.HandleFunc(VersionedPath("/containers/{name}/kill"), s.APIHandler(compat.KillContainer)).Methods(http.MethodPost)
	// Added non version path to URI to support docker non versioned paths
	r.HandleFunc("/containers/{name}/kill", s.APIHandler(compat.KillContainer)).Methods(http.MethodPost)
	// swagger:operation GET /containers/{name}/logs compat logsFromContainer
	// ---
	// tags:
	//   - containers (compat)
	// summary: Get container logs
	// description: Get stdout and stderr logs from a container.
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	//  - in: query
	//    name: follow
	//    type: boolean
	//    description: Keep connection after returning logs.
	//  - in: query
	//    name: stdout
	//    type: boolean
	//    description: Return logs from stdout
	//  - in: query
	//    name: stderr
	//    type: boolean
	//    description: Return logs from stderr
	//  - in: query
	//    name: since
	//    type:  string
	//    description: Only return logs since this time, as a UNIX timestamp
	//  - in: query
	//    name: until
	//    type:  string
	//    description: Only return logs before this time, as a UNIX timestamp
	//  - in: query
	//    name: timestamps
	//    type: boolean
	//    default: false
	//    description: Add timestamps to every log line
	//  - in: query
	//    name: tail
	//    type: string
	//    description: Only return this number of log lines from the end of the logs
	//    default: all
	// produces:
	// - application/json
	// responses:
	//   200:
	//     description:  logs returned as a stream in response body.
	//   404:
	//      $ref: "#/responses/NoSuchContainer"
	//   500:
	//      $ref: "#/responses/InternalError"
	r.HandleFunc(VersionedPath("/containers/{name}/logs"), s.APIHandler(compat.LogsFromContainer)).Methods(http.MethodGet)
	// Added non version path to URI to support docker non versioned paths
	r.HandleFunc("/containers/{name}/logs", s.APIHandler(compat.LogsFromContainer)).Methods(http.MethodGet)
	// swagger:operation POST /containers/{name}/pause compat pauseContainer
	// ---
	// tags:
	//   - containers (compat)
	// summary: Pause container
	// description: Use the cgroups freezer to suspend all processes in a container.
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	// produces:
	// - application/json
	// responses:
	//   204:
	//     description: no error
	//   404:
	//     $ref: "#/responses/NoSuchContainer"
	//   500:
	//     $ref: "#/responses/InternalError"
	r.HandleFunc(VersionedPath("/containers/{name}/pause"), s.APIHandler(compat.PauseContainer)).Methods(http.MethodPost)
	// Added non version path to URI to support docker non versioned paths
	r.HandleFunc("/containers/{name}/pause", s.APIHandler(compat.PauseContainer)).Methods(http.MethodPost)
	r.HandleFunc(VersionedPath("/containers/{name}/rename"), s.APIHandler(compat.UnsupportedHandler)).Methods(http.MethodPost)
	// Added non version path to URI to support docker non versioned paths
	r.HandleFunc("/containers/{name}/rename", s.APIHandler(compat.UnsupportedHandler)).Methods(http.MethodPost)
	// swagger:operation POST /containers/{name}/restart compat restartContainer
	// ---
	// tags:
	//   - containers (compat)
	// summary: Restart container
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	//  - in: query
	//    name: t
	//    type: integer
	//    description: timeout before sending kill signal to container
	// produces:
	// - application/json
	// responses:
	//   204:
	//     description: no error
	//   404:
	//     $ref: "#/responses/NoSuchContainer"
	//   500:
	//     $ref: "#/responses/InternalError"
	r.HandleFunc(VersionedPath("/containers/{name}/restart"), s.APIHandler(compat.RestartContainer)).Methods(http.MethodPost)
	// Added non version path to URI to support docker non versioned paths
	r.HandleFunc("/containers/{name}/restart", s.APIHandler(compat.RestartContainer)).Methods(http.MethodPost)
	// swagger:operation POST /containers/{name}/start compat startContainer
	// ---
	// tags:
	//   - containers (compat)
	// summary: Start a container
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	//  - in: query
	//    name: detachKeys
	//    type: string
	//    description: "Override the key sequence for detaching a container. Format is a single character [a-Z] or ctrl-<value> where <value> is one of: a-z, @, ^, [, , or _."
	//    default: ctrl-p,ctrl-q
	// produces:
	// - application/json
	// responses:
	//   204:
	//     description: no error
	//   304:
	//     $ref: "#/responses/ContainerAlreadyStartedError"
	//   404:
	//     $ref: "#/responses/NoSuchContainer"
	//   500:
	//     $ref: "#/responses/InternalError"
	r.HandleFunc(VersionedPath("/containers/{name}/start"), s.APIHandler(compat.StartContainer)).Methods(http.MethodPost)
	// Added non version path to URI to support docker non versioned paths
	r.HandleFunc("/containers/{name}/start", s.APIHandler(compat.StartContainer)).Methods(http.MethodPost)
	// swagger:operation GET /containers/{name}/stats compat statsContainer
	// ---
	// tags:
	//   - containers (compat)
	// summary: Get stats for a container
	// description: This returns a live stream of a container’s resource usage statistics.
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	//  - in: query
	//    name: stream
	//    type: boolean
	//    default: true
	//    description: Stream the output
	// produces:
	// - application/json
	// responses:
	//   200:
	//     description: OK
	//   404:
	//     $ref: "#/responses/NoSuchContainer"
	//   500:
	//     $ref: "#/responses/InternalError"
	r.HandleFunc(VersionedPath("/containers/{name}/stats"), s.APIHandler(compat.StatsContainer)).Methods(http.MethodGet)
	// Added non version path to URI to support docker non versioned paths
	r.HandleFunc("/containers/{name}/stats", s.APIHandler(compat.StatsContainer)).Methods(http.MethodGet)
	// swagger:operation POST /containers/{name}/stop compat stopContainer
	// ---
	// tags:
	//   - containers (compat)
	// summary: Stop a container
	// description: Stop a container
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	//  - in: query
	//    name: t
	//    type: integer
	//    description: number of seconds to wait before killing container
	// produces:
	// - application/json
	// responses:
	//   204:
	//     description: no error
	//   304:
	//     $ref: "#/responses/ContainerAlreadyStoppedError"
	//   404:
	//     $ref: "#/responses/NoSuchContainer"
	//   500:
	//     $ref: "#/responses/InternalError"
	r.HandleFunc(VersionedPath("/containers/{name}/stop"), s.APIHandler(compat.StopContainer)).Methods(http.MethodPost)
	// Added non version path to URI to support docker non versioned paths
	r.HandleFunc("/containers/{name}/stop", s.APIHandler(compat.StopContainer)).Methods(http.MethodPost)
	// swagger:operation GET /containers/{name}/top compat topContainer
	// ---
	// tags:
	//   - containers (compat)
	// summary: List processes running inside a container
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	//  - in: query
	//    name: ps_args
	//    type: string
	//    description: arguments to pass to ps such as aux. Requires ps(1) to be installed in the container if no ps(1) compatible AIX descriptors are used.
	// produces:
	// - application/json
	// responses:
	//   200:
	//     $ref: "#/responses/DocsContainerTopResponse"
	//   404:
	//     $ref: "#/responses/NoSuchContainer"
	//   500:
	//     $ref: "#/responses/InternalError"
	r.HandleFunc(VersionedPath("/containers/{name}/top"), s.APIHandler(compat.TopContainer)).Methods(http.MethodGet)
	// Added non version path to URI to support docker non versioned paths
	r.HandleFunc("/containers/{name}/top", s.APIHandler(compat.TopContainer)).Methods(http.MethodGet)
	// swagger:operation POST /containers/{name}/unpause compat unpauseContainer
	// ---
	// tags:
	//   - containers (compat)
	// summary: Unpause container
	// description: Resume a paused container
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	// produces:
	// - application/json
	// responses:
	//   204:
	//     description: no error
	//   404:
	//     $ref: "#/responses/NoSuchContainer"
	//   500:
	//     $ref: "#/responses/InternalError"
	r.HandleFunc(VersionedPath("/containers/{name}/unpause"), s.APIHandler(compat.UnpauseContainer)).Methods(http.MethodPost)
	// Added non version path to URI to support docker non versioned paths
	r.HandleFunc("/containers/{name}/unpause", s.APIHandler(compat.UnpauseContainer)).Methods(http.MethodPost)
	// swagger:operation POST /containers/{name}/wait compat waitContainer
	// ---
	// tags:
	//   - containers (compat)
	// summary: Wait on a container
	// description: Block until a container stops or given condition is met.
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	//  - in: query
	//    name: condition
	//    type: string
	//    description: |
	//      wait until container is to a given condition. default is stopped. valid conditions are:
	//        - configured
	//        - created
	//        - exited
	//        - paused
	//        - running
	//        - stopped
	// produces:
	// - application/json
	// responses:
	//   200:
	//     $ref: "#/responses/ContainerWaitResponse"
	//   404:
	//     $ref: "#/responses/NoSuchContainer"
	//   500:
	//     $ref: "#/responses/InternalError"
	r.HandleFunc(VersionedPath("/containers/{name}/wait"), s.APIHandler(compat.WaitContainer)).Methods(http.MethodPost)
	// Added non version path to URI to support docker non versioned paths
	r.HandleFunc("/containers/{name}/wait", s.APIHandler(compat.WaitContainer)).Methods(http.MethodPost)
	// swagger:operation POST /containers/{name}/attach compat attachContainer
	// ---
	// tags:
	//   - containers (compat)
	// summary: Attach to a container
	// description: Hijacks the connection to forward the container's standard streams to the client.
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	//  - in: query
	//    name: detachKeys
	//    required: false
	//    type: string
	//    description: keys to use for detaching from the container
	//  - in: query
	//    name: logs
	//    required: false
	//    type: boolean
	//    description: Stream all logs from the container across the connection. Happens before streaming attach (if requested). At least one of logs or stream must be set
	//  - in: query
	//    name: stream
	//    required: false
	//    type: boolean
	//    default: true
	//    description: Attach to the container. If unset, and logs is set, only the container's logs will be sent. At least one of stream or logs must be set
	//  - in: query
	//    name: stdout
	//    required: false
	//    type: boolean
	//    description: Attach to container STDOUT
	//  - in: query
	//    name: stderr
	//    required: false
	//    type: boolean
	//    description: Attach to container STDERR
	//  - in: query
	//    name: stdin
	//    required: false
	//    type: boolean
	//    description: Attach to container STDIN
	// produces:
	// - application/json
	// responses:
	//   101:
	//     description: No error, connection has been hijacked for transporting streams.
	//   400:
	//     $ref: "#/responses/BadParamError"
	//   404:
	//     $ref: "#/responses/NoSuchContainer"
	//   500:
	//     $ref: "#/responses/InternalError"
	r.HandleFunc(VersionedPath("/containers/{name}/attach"), s.APIHandler(compat.AttachContainer)).Methods(http.MethodPost)
	// Added non version path to URI to support docker non versioned paths
	r.HandleFunc("/containers/{name}/attach", s.APIHandler(compat.AttachContainer)).Methods(http.MethodPost)
	// swagger:operation POST /containers/{name}/resize compat resizeContainer
	// ---
	// tags:
	//  - containers (compat)
	// summary: Resize a container's TTY
	// description: Resize the terminal attached to a container (for use with Attach).
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	//  - in: query
	//    name: h
	//    type: integer
	//    required: false
	//    description: Height to set for the terminal, in characters
	//  - in: query
	//    name: w
	//    type: integer
	//    required: false
	//    description: Width to set for the terminal, in characters
	// produces:
	// - application/json
	// responses:
	//   200:
	//     $ref: "#/responses/ok"
	//   404:
	//     $ref: "#/responses/NoSuchContainer"
	//   500:
	//     $ref: "#/responses/InternalError"
	r.HandleFunc(VersionedPath("/containers/{name}/resize"), s.APIHandler(compat.ResizeTTY)).Methods(http.MethodPost)
	// Added non version path to URI to support docker non versioned paths
	r.HandleFunc("/containers/{name}/resize", s.APIHandler(compat.ResizeTTY)).Methods(http.MethodPost)
	// swagger:operation GET /containers/{name}/export compat exportContainer
	// ---
	// tags:
	//   - containers (compat)
	// summary: Export a container
	// description: Export the contents of a container as a tarball.
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	// produces:
	// - application/json
	// responses:
	//   200:
	//     description: tarball is returned in body
	//   404:
	//     $ref: "#/responses/NoSuchContainer"
	//   500:
	//     $ref: "#/responses/InternalError"
	r.HandleFunc(VersionedPath("/containers/{name}/export"), s.APIHandler(compat.ExportContainer)).Methods(http.MethodGet)
	r.HandleFunc("/containers/{name}/export", s.APIHandler(compat.ExportContainer)).Methods(http.MethodGet)

	/*
		libpod endpoints
	*/

	// swagger:operation POST /libpod/containers/create libpod libpodCreateContainer
	// ---
	//   summary: Create a container
	//   tags:
	//    - containers
	//   produces:
	//   - application/json
	//   parameters:
	//    - in: body
	//      name: create
	//      description: attributes for creating a container
	//      schema:
	//        $ref: "#/definitions/SpecGenerator"
	//   responses:
	//     201:
	//       $ref: "#/responses/ContainerCreateResponse"
	//     400:
	//       $ref: "#/responses/BadParamError"
	//     404:
	//       $ref: "#/responses/NoSuchContainer"
	//     409:
	//       $ref: "#/responses/ConflictError"
	//     500:
	//       $ref: "#/responses/InternalError"
	r.HandleFunc(VersionedPath("/libpod/containers/create"), s.APIHandler(libpod.CreateContainer)).Methods(http.MethodPost)
	// swagger:operation GET /libpod/containers/json libpod libpodListContainers
	// ---
	// tags:
	//  - containers
	// summary: List containers
	// description: Returns a list of containers
	// parameters:
	//  - in: query
	//    name: all
	//    type: boolean
	//    default: false
	//    description: Return all containers. By default, only running containers are shown
	//  - in: query
	//    name: limit
	//    description: Return this number of most recently created containers, including non-running ones.
	//    type: integer
	//  - in: query
	//    name: namespace
	//    type: boolean
	//    description: Include namespace information
	//    default: false
	//  - in: query
	//    name: pod
	//    type: boolean
	//    default: false
	//    description: Include Pod ID and Name if applicable
	//  - in: query
	//    name: size
	//    type: boolean
	//    default: false
	//    description: Return the size of container as fields SizeRw and SizeRootFs.
	//  - in: query
	//    name: sync
	//    type: boolean
	//    default: false
	//    description: Sync container state with OCI runtime
	//  - in: query
	//    name: filters
	//    type: string
	//    description: |
	//        A JSON encoded value of the filters (a `map[string][]string`) to process on the containers list. Available filters:
	//        - `ancestor`=(`<image-name>[:<tag>]`, `<image id>`, or `<image@digest>`)
	//        - `before`=(`<container id>` or `<container name>`)
	//        - `expose`=(`<port>[/<proto>]` or `<startport-endport>/[<proto>]`)
	//        - `exited=<int>` containers with exit code of `<int>`
	//        - `health`=(`starting`, `healthy`, `unhealthy` or `none`)
	//        - `id=<ID>` a container's ID
	//        - `is-task`=(`true` or `false`)
	//        - `label`=(`key` or `"key=value"`) of an container label
	//        - `name=<name>` a container's name
	//        - `network`=(`<network id>` or `<network name>`)
	//        - `publish`=(`<port>[/<proto>]` or `<startport-endport>/[<proto>]`)
	//        - `since`=(`<container id>` or `<container name>`)
	//        - `status`=(`created`, `restarting`, `running`, `removing`, `paused`, `exited` or `dead`)
	//        - `volume`=(`<volume name>` or `<mount point destination>`)
	// produces:
	// - application/json
	// responses:
	//   200:
	//     $ref: "#/responses/ListContainers"
	//   400:
	//     $ref: "#/responses/BadParamError"
	//   500:
	//     $ref: "#/responses/InternalError"
	r.HandleFunc(VersionedPath("/libpod/containers/json"), s.APIHandler(libpod.ListContainers)).Methods(http.MethodGet)
	// swagger:operation POST  /libpod/containers/prune libpod libpodPruneContainers
	// ---
	// tags:
	//   - containers
	// summary: Delete stopped containers
	// description: Remove containers not in use
	// parameters:
	//  - in: query
	//    name: filters
	//    type: string
	//    description:  |
	//      Filters to process on the prune list, encoded as JSON (a `map[string][]string`).  Available filters:
	//       - `until=<timestamp>` Prune containers created before this timestamp. The `<timestamp>` can be Unix timestamps, date formatted timestamps, or Go duration strings (e.g. `10m`, `1h30m`) computed relative to the daemon machine’s time.
	//       - `label` (`label=<key>`, `label=<key>=<value>`, `label!=<key>`, or `label!=<key>=<value>`) Prune containers with (or without, in case `label!=...` is used) the specified labels.
	// produces:
	// - application/json
	// responses:
	//   200:
	//     $ref: "#/responses/DocsLibpodPruneResponse"
	//   500:
	//     $ref: "#/responses/InternalError"
	r.HandleFunc(VersionedPath("/libpod/containers/prune"), s.APIHandler(compat.PruneContainers)).Methods(http.MethodPost)
	// swagger:operation GET /libpod/containers/showmounted libpod libpodShowMountedContainers
	// ---
	// tags:
	//  - containers
	// summary: Show mounted containers
	// description: Lists all mounted containers mount points
	// produces:
	// - application/json
	// responses:
	//   200:
	//     description: mounted containers
	//     schema:
	//      type: object
	//      additionalProperties:
	//       type: string
	//   500:
	//      $ref: "#/responses/InternalError"
	r.HandleFunc(VersionedPath("/libpod/containers/showmounted"), s.APIHandler(libpod.ShowMountedContainers)).Methods(http.MethodGet)
	// swagger:operation DELETE /libpod/containers/{name} libpod libpodRemoveContainer
	// ---
	// tags:
	//  - containers
	// summary: Delete container
	// description: Delete container
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	//  - in: query
	//    name: force
	//    type: boolean
	//    description: need something
	//  - in: query
	//    name: v
	//    type: boolean
	//    description: delete volumes
	// produces:
	// - application/json
	// responses:
	//   204:
	//     description: no error
	//   400:
	//     $ref: "#/responses/BadParamError"
	//   404:
	//     $ref: "#/responses/NoSuchContainer"
	//   409:
	//     $ref: "#/responses/ConflictError"
	//   500:
	//     $ref: "#/responses/InternalError"
	r.HandleFunc(VersionedPath("/libpod/containers/{name}"), s.APIHandler(compat.RemoveContainer)).Methods(http.MethodDelete)
	// swagger:operation GET /libpod/containers/{name}/json libpod libpodGetContainer
	// ---
	// tags:
	//  - containers
	// summary: Inspect container
	// description: Return low-level information about a container.
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	//  - in: query
	//    name: size
	//    type: boolean
	//    description: display filesystem usage
	// produces:
	// - application/json
	// responses:
	//   200:
	//     $ref: "#/responses/LibpodInspectContainerResponse"
	//   404:
	//     $ref: "#/responses/NoSuchContainer"
	//   500:
	//     $ref: "#/responses/InternalError"
	r.HandleFunc(VersionedPath("/libpod/containers/{name}/json"), s.APIHandler(libpod.GetContainer)).Methods(http.MethodGet)
	// swagger:operation POST /libpod/containers/{name}/kill libpod libpodKillContainer
	// ---
	// tags:
	//  - containers
	// summary: Kill container
	// description: send a signal to a container, defaults to killing the container
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	//  - in: query
	//    name: signal
	//    type: string
	//    default: TERM
	//    description: signal to be sent to container, either by integer or SIG_ name
	// produces:
	// - application/json
	// responses:
	//   204:
	//     description: no error
	//   404:
	//     $ref: "#/responses/NoSuchContainer"
	//   409:
	//     $ref: "#/responses/ConflictError"
	//   500:
	//     $ref: "#/responses/InternalError"
	r.HandleFunc(VersionedPath("/libpod/containers/{name}/kill"), s.APIHandler(compat.KillContainer)).Methods(http.MethodPost)
	// swagger:operation POST /libpod/containers/{name}/mount libpod libpodMountContainer
	// ---
	// tags:
	//  - containers
	// summary: Mount a container
	// description: Mount a container to the filesystem
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	// produces:
	// - application/json
	// responses:
	//   200:
	//     description: mounted container
	//     schema:
	//      description: id
	//      type: string
	//      example: /var/lib/containers/storage/overlay/f3f693bd88872a1e3193f4ebb925f4c282e8e73aadb8ab3e7492754dda3a02a4/merged
	//   404:
	//     $ref: "#/responses/NoSuchContainer"
	//   500:
	//     $ref: "#/responses/InternalError"
	r.HandleFunc(VersionedPath("/libpod/containers/{name}/mount"), s.APIHandler(libpod.MountContainer)).Methods(http.MethodPost)
	// swagger:operation POST /libpod/containers/{name}/unmount libpod libpodUnmountContainer
	// ---
	// tags:
	//  - containers
	// summary: Unmount a container
	// description: Unmount a container from the filesystem
	// produces:
	// - application/json
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	// responses:
	//   204:
	//     description: ok
	//   404:
	//     $ref: "#/responses/NoSuchContainer"
	//   500:
	//     $ref: "#/responses/InternalError"
	r.HandleFunc(VersionedPath("/libpod/containers/{name}/unmount"), s.APIHandler(libpod.UnmountContainer)).Methods(http.MethodPost)
	// swagger:operation GET /libpod/containers/{name}/logs libpod libpodLogsFromContainer
	// ---
	// tags:
	//   - containers
	// summary: Get container logs
	// description: Get stdout and stderr logs from a container.
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	//  - in: query
	//    name: follow
	//    type: boolean
	//    description: Keep connection after returning logs.
	//  - in: query
	//    name: stdout
	//    type: boolean
	//    description: Return logs from stdout
	//  - in: query
	//    name: stderr
	//    type: boolean
	//    description: Return logs from stderr
	//  - in: query
	//    name: since
	//    type:  string
	//    description: Only return logs since this time, as a UNIX timestamp
	//  - in: query
	//    name: until
	//    type:  string
	//    description: Only return logs before this time, as a UNIX timestamp
	//  - in: query
	//    name: timestamps
	//    type: boolean
	//    default: false
	//    description: Add timestamps to every log line
	//  - in: query
	//    name: tail
	//    type: string
	//    description: Only return this number of log lines from the end of the logs
	//    default: all
	// produces:
	// - application/json
	// responses:
	//   200:
	//     description:  logs returned as a stream in response body.
	//   404:
	//      $ref: "#/responses/NoSuchContainer"
	//   500:
	//      $ref: "#/responses/InternalError"
	r.HandleFunc(VersionedPath("/libpod/containers/{name}/logs"), s.APIHandler(compat.LogsFromContainer)).Methods(http.MethodGet)
	// swagger:operation POST /libpod/containers/{name}/pause libpod libpodPauseContainer
	// ---
	// tags:
	//  - containers
	// summary: Pause a container
	// description: Use the cgroups freezer to suspend all processes in a container.
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	// produces:
	// - application/json
	// responses:
	//   204:
	//     description: no error
	//   404:
	//     "$ref": "#/responses/NoSuchContainer"
	//   500:
	//     "$ref": "#/responses/InternalError"
	r.HandleFunc(VersionedPath("/libpod/containers/{name}/pause"), s.APIHandler(compat.PauseContainer)).Methods(http.MethodPost)
	// swagger:operation POST /libpod/containers/{name}/restart libpod libpodRestartContainer
	// ---
	// tags:
	//  - containers
	// summary: Restart a container
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	//  - in: query
	//    name: t
	//    type: integer
	//    description: timeout before sending kill signal to container
	// produces:
	// - application/json
	// responses:
	//   204:
	//     description: no error
	//   404:
	//     $ref: "#/responses/NoSuchContainer"
	//   500:
	//     $ref: "#/responses/InternalError"
	r.HandleFunc(VersionedPath("/libpod/containers/{name}/restart"), s.APIHandler(compat.RestartContainer)).Methods(http.MethodPost)
	// swagger:operation POST /libpod/containers/{name}/start libpod libpodStartContainer
	// ---
	// tags:
	//  - containers
	// summary: Start a container
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	//  - in: query
	//    name: detachKeys
	//    type: string
	//    description: "Override the key sequence for detaching a container. Format is a single character [a-Z] or ctrl-<value> where <value> is one of: a-z, @, ^, [, , or _."
	//    default: ctrl-p,ctrl-q
	// produces:
	// - application/json
	// responses:
	//   204:
	//     description: no error
	//   304:
	//     $ref: "#/responses/ContainerAlreadyStartedError"
	//   404:
	//     $ref: "#/responses/NoSuchContainer"
	//   500:
	//     $ref: "#/responses/InternalError"
	r.HandleFunc(VersionedPath("/libpod/containers/{name}/start"), s.APIHandler(compat.StartContainer)).Methods(http.MethodPost)
	// swagger:operation GET /libpod/containers/{name}/stats libpod libpodStatsContainer
	// ---
	// tags:
	//  - containers
	// summary: Get stats for a container
	// description: This returns a live stream of a container’s resource usage statistics.
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	//  - in: query
	//    name: stream
	//    type: boolean
	//    default: true
	//    description: Stream the output
	// produces:
	// - application/json
	// responses:
	//   200:
	//     description: no error
	//   404:
	//     $ref: "#/responses/NoSuchContainer"
	//   500:
	//     $ref: "#/responses/InternalError"
	r.HandleFunc(VersionedPath("/libpod/containers/{name}/stats"), s.APIHandler(compat.StatsContainer)).Methods(http.MethodGet)
	// swagger:operation GET /libpod/containers/{name}/top libpod libpodTopContainer
	// ---
	// tags:
	//  - containers
	// summary: List processes
	// description: List processes running inside a container
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: |
	//      Name of container to query for processes
	//      (As of version 1.xx)
	//  - in: query
	//    name: stream
	//    type: boolean
	//    default: true
	//    description: Stream the output
	//  - in: query
	//    name: ps_args
	//    type: string
	//    default: -ef
	//    description: arguments to pass to ps such as aux. Requires ps(1) to be installed in the container if no ps(1) compatible AIX descriptors are used.
	// produces:
	// - application/json
	// responses:
	//   200:
	//     $ref: "#/responses/DocsContainerTopResponse"
	//   404:
	//     $ref: "#/responses/NoSuchContainer"
	//   500:
	//     $ref: "#/responses/InternalError"
	r.HandleFunc(VersionedPath("/libpod/containers/{name}/top"), s.APIHandler(compat.TopContainer)).Methods(http.MethodGet)
	// swagger:operation POST /libpod/containers/{name}/unpause libpod libpodUnpauseContainer
	// ---
	// tags:
	//  - containers
	// summary: Unpause Container
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	// produces:
	// - application/json
	// responses:
	//   204:
	//     description: no error
	//   404:
	//     $ref: "#/responses/NoSuchContainer"
	//   500:
	//     $ref: "#/responses/InternalError"
	r.HandleFunc(VersionedPath("/libpod/containers/{name}/unpause"), s.APIHandler(compat.UnpauseContainer)).Methods(http.MethodPost)
	// swagger:operation POST /libpod/containers/{name}/wait libpod libpodWaitContainer
	// ---
	// tags:
	//  - containers
	// summary: Wait on a container
	// description: Wait on a container to met a given condition
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	//  - in: query
	//    name: condition
	//    type: string
	//    description: |
	//      wait until container is to a given condition. default is stopped. valid conditions are:
	//        - configured
	//        - created
	//        - exited
	//        - paused
	//        - running
	//        - stopped
	// produces:
	// - application/json
	// responses:
	//   200:
	//     $ref: "#/responses/ContainerWaitResponse"
	//   404:
	//     $ref: "#/responses/NoSuchContainer"
	//   500:
	//     $ref: "#/responses/InternalError"
	r.HandleFunc(VersionedPath("/libpod/containers/{name}/wait"), s.APIHandler(libpod.WaitContainer)).Methods(http.MethodPost)
	// swagger:operation GET /libpod/containers/{name}/exists libpod libpodContainerExists
	// ---
	// tags:
	//  - containers
	// summary: Check if container exists
	// description: Quick way to determine if a container exists by name or ID
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	// produces:
	// - application/json
	// responses:
	//   204:
	//     description: container exists
	//   404:
	//     $ref: "#/responses/NoSuchContainer"
	//   500:
	//     $ref: "#/responses/InternalError"
	r.HandleFunc(VersionedPath("/libpod/containers/{name}/exists"), s.APIHandler(libpod.ContainerExists)).Methods(http.MethodGet)
	// swagger:operation POST /libpod/containers/{name}/stop libpod libpodStopContainer
	// ---
	// tags:
	//  - containers
	// summary: Stop a container
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	//  - in: query
	//    name: t
	//    type: integer
	//    description: number of seconds to wait before killing container
	// produces:
	// - application/json
	// responses:
	//   204:
	//     description: no error
	//   304:
	//     $ref: "#/responses/ContainerAlreadyStoppedError"
	//   404:
	//     $ref: "#/responses/NoSuchContainer"
	//   500:
	//     $ref: "#/responses/InternalError"
	r.HandleFunc(VersionedPath("/libpod/containers/{name}/stop"), s.APIHandler(compat.StopContainer)).Methods(http.MethodPost)
	// swagger:operation POST /libpod/containers/{name}/attach libpod libpodAttachContainer
	// ---
	// tags:
	//   - containers
	// summary: Attach to a container
	// description: Hijacks the connection to forward the container's standard streams to the client.
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	//  - in: query
	//    name: detachKeys
	//    required: false
	//    type: string
	//    description: keys to use for detaching from the container
	//  - in: query
	//    name: logs
	//    required: false
	//    type: boolean
	//    description: Stream all logs from the container across the connection. Happens before streaming attach (if requested). At least one of logs or stream must be set
	//  - in: query
	//    name: stream
	//    required: false
	//    type: boolean
	//    default: true
	//    description: Attach to the container. If unset, and logs is set, only the container's logs will be sent. At least one of stream or logs must be set
	//  - in: query
	//    name: stdout
	//    required: false
	//    type: boolean
	//    description: Attach to container STDOUT
	//  - in: query
	//    name: stderr
	//    required: false
	//    type: boolean
	//    description: Attach to container STDERR
	//  - in: query
	//    name: stdin
	//    required: false
	//    type: boolean
	//    description: Attach to container STDIN
	// produces:
	// - application/json
	// responses:
	//   101:
	//     description: No error, connection has been hijacked for transporting streams.
	//   400:
	//     $ref: "#/responses/BadParamError"
	//   404:
	//     $ref: "#/responses/NoSuchContainer"
	//   500:
	//     $ref: "#/responses/InternalError"
	r.HandleFunc(VersionedPath("/libpod/containers/{name}/attach"), s.APIHandler(compat.AttachContainer)).Methods(http.MethodPost)
	// swagger:operation POST /libpod/containers/{name}/resize libpod libpodResizeContainer
	// ---
	// tags:
	//  - containers
	// summary: Resize a container's TTY
	// description: Resize the terminal attached to a container (for use with Attach).
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	//  - in: query
	//    name: h
	//    type: integer
	//    required: false
	//    description: Height to set for the terminal, in characters
	//  - in: query
	//    name: w
	//    type: integer
	//    required: false
	//    description: Width to set for the terminal, in characters
	// produces:
	// - application/json
	// responses:
	//   200:
	//     $ref: "#/responses/ok"
	//   404:
	//     $ref: "#/responses/NoSuchContainer"
	//   500:
	//     $ref: "#/responses/InternalError"
	r.HandleFunc(VersionedPath("/libpod/containers/{name}/resize"), s.APIHandler(compat.ResizeTTY)).Methods(http.MethodPost)
	// swagger:operation GET /libpod/containers/{name}/export libpod libpodExportContainer
	// ---
	// tags:
	//   - containers
	// summary: Export a container
	// description: Export the contents of a container as a tarball.
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	// produces:
	// - application/json
	// responses:
	//   200:
	//     description: tarball is returned in body
	//   404:
	//     $ref: "#/responses/NoSuchContainer"
	//   500:
	//     $ref: "#/responses/InternalError"
	r.HandleFunc(VersionedPath("/libpod/containers/{name}/export"), s.APIHandler(compat.ExportContainer)).Methods(http.MethodGet)
	// swagger:operation POST /libpod/containers/{name}/checkpoint libpod libpodCheckpointContainer
	// ---
	// tags:
	//   - containers
	// summary: Checkpoint a container
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	//  - in: query
	//    name: keep
	//    type: boolean
	//    description: keep all temporary checkpoint files
	//  - in: query
	//    name: leaveRunning
	//    type: boolean
	//    description: leave the container running after writing checkpoint to disk
	//  - in: query
	//    name: tcpEstablished
	//    type: boolean
	//    description: checkpoint a container with established TCP connections
	//  - in: query
	//    name: export
	//    type: boolean
	//    description:  export the checkpoint image to a tar.gz
	//  - in: query
	//    name: ignoreRootFS
	//    type: boolean
	//    description: do not include root file-system changes when exporting
	// produces:
	// - application/json
	// responses:
	//   200:
	//     description: tarball is returned in body if exported
	//   404:
	//     $ref: "#/responses/NoSuchContainer"
	//   500:
	//     $ref: "#/responses/InternalError"
	r.HandleFunc(VersionedPath("/libpod/containers/{name}/checkpoint"), s.APIHandler(libpod.Checkpoint)).Methods(http.MethodPost)
	// swagger:operation POST /libpod/containers/{name}/restore libpod libpodRestoreContainer
	// ---
	// tags:
	//   - containers
	// summary: Restore a container
	// description: Restore a container from a checkpoint.
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or id of the container
	//  - in: query
	//    name: name
	//    type: string
	//    description: the name of the container when restored from a tar. can only be used with import
	//  - in: query
	//    name: keep
	//    type: boolean
	//    description: keep all temporary checkpoint files
	//  - in: query
	//    name: leaveRunning
	//    type: boolean
	//    description: leave the container running after writing checkpoint to disk
	//  - in: query
	//    name: tcpEstablished
	//    type: boolean
	//    description: checkpoint a container with established TCP connections
	//  - in: query
	//    name: import
	//    type: boolean
	//    description:  import the restore from a checkpoint tar.gz
	//  - in: query
	//    name: ignoreRootFS
	//    type: boolean
	//    description: do not include root file-system changes when exporting
	//  - in: query
	//    name: ignoreStaticIP
	//    type: boolean
	//    description: ignore IP address if set statically
	//  - in: query
	//    name: ignoreStaticMAC
	//    type: boolean
	//    description: ignore MAC address if set statically
	// produces:
	// - application/json
	// responses:
	//   200:
	//     description: tarball is returned in body if exported
	//   404:
	//     $ref: "#/responses/NoSuchContainer"
	//   500:
	//     $ref: "#/responses/InternalError"
	r.HandleFunc(VersionedPath("/libpod/containers/{name}/restore"), s.APIHandler(libpod.Restore)).Methods(http.MethodPost)
	// swagger:operation GET /containers/{name}/changes libpod libpodChangesContainer
	// swagger:operation GET /libpod/containers/{name}/changes compat changesContainer
	// ---
	// tags:
	//   - containers
	//   - containers (compat)
	// summary: Report on changes to container's filesystem; adds, deletes or modifications.
	// description: |
	//   Returns which files in a container's filesystem have been added, deleted, or modified. The Kind of modification can be one of:
	//
	//   0: Modified
	//   1: Added
	//   2: Deleted
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or id of the container
	// responses:
	//   200:
	//     description: Array of Changes
	//     content:
	//       application/json:
	//       schema:
	//         $ref: "#/responses/Changes"
	//   404:
	//     $ref: "#/responses/NoSuchContainer"
	//   500:
	//     $ref: "#/responses/InternalError"
	r.HandleFunc(VersionedPath("/containers/{name}/changes"), s.APIHandler(compat.Changes)).Methods(http.MethodGet)
	r.HandleFunc("/containers/{name}/changes", s.APIHandler(compat.Changes)).Methods(http.MethodGet)
	r.HandleFunc(VersionedPath("/libpod/containers/{name}/changes"), s.APIHandler(compat.Changes)).Methods(http.MethodGet)
	// swagger:operation POST /libpod/containers/{name}/init libpod libpodInitContainer
	// ---
	// tags:
	//  - containers
	// summary: Initialize a container
	// description: Performs all tasks necessary for initializing the container but does not start the container.
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the container
	// produces:
	// - application/json
	// responses:
	//   204:
	//     description: no error
	//   304:
	//     description: container already initialized
	//   404:
	//     $ref: "#/responses/NoSuchContainer"
	//   500:
	//     $ref: "#/responses/InternalError"
	r.HandleFunc(VersionedPath("/libpod/containers/{name}/init"), s.APIHandler(libpod.InitContainer)).Methods(http.MethodPost)
	return nil
}
