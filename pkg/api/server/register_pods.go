package server

import (
	"net/http"

	"github.com/containers/podman/v4/pkg/api/handlers/libpod"
	"github.com/gorilla/mux"
)

func (s *APIServer) registerPodsHandlers(r *mux.Router) error {
	// swagger:operation GET /libpod/pods/json pods PodListLibpod
	// ---
	// summary: List pods
	// produces:
	// - application/json
	// parameters:
	// - in: query
	//   name: filters
	//   type: string
	//   description: |
	//      JSON encoded value of the filters (a map[string][]string) to process on the pods list. Available filters:
	//        - `id=<pod-id>` Matches all of pod id.
	//        - `label=<key>` or `label=<key>:<value>` Matches pods based on the presence of a label alone or a label and a value.
	//        - `name=<pod-name>` Matches all of pod name.
	//        - `until=<timestamp>` List pods created before this timestamp. The `<timestamp>` can be Unix timestamps, date formatted timestamps, or Go duration strings (e.g. `10m`, `1h30m`) computed relative to the daemon machine’s time.
	//        - `status=<pod-status>` Pod's status: `stopped`, `running`, `paused`, `exited`, `dead`, `created`, `degraded`.
	//        - `network=<pod-network>` Name or full ID of network.
	//        - `ctr-names=<pod-ctr-names>` Container name within the pod.
	//        - `ctr-ids=<pod-ctr-ids>` Container ID within the pod.
	//        - `ctr-status=<pod-ctr-status>` Container status within the pod.
	//        - `ctr-number=<pod-ctr-number>` Number of containers in the pod.
	// responses:
	//   200:
	//     $ref: "#/responses/ListPodsResponse"
	//   400:
	//     $ref: "#/responses/BadParamError"
	//   500:
	//     $ref: "#/responses/InternalError"
	r.Handle(VersionedPath("/libpod/pods/json"), s.APIHandler(libpod.Pods)).Methods(http.MethodGet)
	// swagger:operation POST /libpod/pods/create pods PodCreateLibpod
	// ---
	// summary: Create a pod
	// produces:
	// - application/json
	// parameters:
	// - in: body
	//   name: create
	//   description: attributes for creating a pod
	//   schema:
	//     $ref: "#/definitions/PodSpecGenerator"
	// responses:
	//   201:
	//     schema:
	//       $ref: "#/definitions/IDResponse"
	//   400:
	//     $ref: "#/responses/BadParamError"
	//   409:
	//     description: status conflict
	//     schema:
	//       type: string
	//       description: message describing error
	//   500:
	//     $ref: "#/responses/InternalError"
	r.Handle(VersionedPath("/libpod/pods/create"), s.APIHandler(libpod.PodCreate)).Methods(http.MethodPost)
	// swagger:operation POST /libpod/pods/prune pods PodPruneLibpod
	// ---
	// summary: Prune unused pods
	// produces:
	// - application/json
	// responses:
	//   200:
	//     $ref: '#/responses/PodPruneReport'
	//   400:
	//     $ref: "#/responses/BadParamError"
	//   409:
	//     description: pod already exists
	//   500:
	//     $ref: "#/responses/InternalError"
	r.Handle(VersionedPath("/libpod/pods/prune"), s.APIHandler(libpod.PodPrune)).Methods(http.MethodPost)
	// swagger:operation DELETE /libpod/pods/{name} pods PodDeleteLibpod
	// ---
	// summary: Remove pod
	// produces:
	// - application/json
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the pod
	//  - in: query
	//    name: force
	//    type: boolean
	//    description : force removal of a running pod by first stopping all containers, then removing all containers in the pod
	// responses:
	//   200:
	//     $ref: '#/responses/PodRmReport'
	//   400:
	//     $ref: "#/responses/BadParamError"
	//   404:
	//     $ref: "#/responses/NoSuchPod"
	//   500:
	//     $ref: "#/responses/InternalError"
	r.Handle(VersionedPath("/libpod/pods/{name}"), s.APIHandler(libpod.PodDelete)).Methods(http.MethodDelete)
	// swagger:operation GET /libpod/pods/{name}/json pods PodInspectLibpod
	// ---
	// summary: Inspect pod
	// produces:
	// - application/json
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the pod
	// responses:
	//   200:
	//     $ref: "#/responses/InspectPodResponse"
	//   404:
	//      $ref: "#/responses/NoSuchPod"
	//   500:
	//      $ref: "#/responses/InternalError"
	r.Handle(VersionedPath("/libpod/pods/{name}/json"), s.APIHandler(libpod.PodInspect)).Methods(http.MethodGet)
	// swagger:operation GET /libpod/pods/{name}/exists pods PodExistsLibpod
	// ---
	// summary: Pod exists
	// description: Check if a pod exists by name or ID
	// produces:
	// - application/json
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the pod
	// responses:
	//   204:
	//     description: pod exists
	//   404:
	//     $ref: "#/responses/NoSuchPod"
	//   500:
	//     $ref: "#/responses/InternalError"
	r.Handle(VersionedPath("/libpod/pods/{name}/exists"), s.APIHandler(libpod.PodExists)).Methods(http.MethodGet)
	// swagger:operation POST /libpod/pods/{name}/kill pods PodKillLibpod
	// ---
	// summary: Kill a pod
	// produces:
	// - application/json
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the pod
	//  - in: query
	//    name: signal
	//    type: string
	//    description: signal to be sent to pod
	//    default: SIGKILL
	// responses:
	//   200:
	//     $ref: "#/responses/PodKillReport"
	//   400:
	//     $ref: "#/responses/BadParamError"
	//   404:
	//     $ref: "#/responses/NoSuchPod"
	//   409:
	//     $ref: "#/responses/PodKillReport"
	//   500:
	//     $ref: "#/responses/InternalError"
	r.Handle(VersionedPath("/libpod/pods/{name}/kill"), s.APIHandler(libpod.PodKill)).Methods(http.MethodPost)
	// swagger:operation POST /libpod/pods/{name}/pause pods PodPauseLibpod
	// ---
	// summary: Pause a pod
	// description: Pause a pod
	// produces:
	// - application/json
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the pod
	// responses:
	//   200:
	//     $ref: '#/responses/PodPauseReport'
	//   404:
	//     $ref: "#/responses/NoSuchPod"
	//   409:
	//     $ref: '#/responses/PodPauseReport'
	//   500:
	//     $ref: "#/responses/InternalError"
	r.Handle(VersionedPath("/libpod/pods/{name}/pause"), s.APIHandler(libpod.PodPause)).Methods(http.MethodPost)
	// swagger:operation POST /libpod/pods/{name}/restart pods PodRestartLibpod
	// ---
	// summary: Restart a pod
	// produces:
	// - application/json
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the pod
	// responses:
	//   200:
	//     $ref: '#/responses/PodRestartReport'
	//   404:
	//     $ref: "#/responses/NoSuchPod"
	//   409:
	//     $ref: "#/responses/PodRestartReport"
	//   500:
	//     $ref: "#/responses/InternalError"
	r.Handle(VersionedPath("/libpod/pods/{name}/restart"), s.APIHandler(libpod.PodRestart)).Methods(http.MethodPost)
	// swagger:operation POST /libpod/pods/{name}/start pods PodStartLibpod
	// ---
	// summary: Start a pod
	// produces:
	// - application/json
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the pod
	// responses:
	//   200:
	//     $ref: '#/responses/PodStartReport'
	//   304:
	//     $ref: "#/responses/PodAlreadyStartedError"
	//   404:
	//     $ref: "#/responses/NoSuchPod"
	//   409:
	//     $ref: '#/responses/PodStartReport'
	//   500:
	//     $ref: "#/responses/InternalError"
	r.Handle(VersionedPath("/libpod/pods/{name}/start"), s.APIHandler(libpod.PodStart)).Methods(http.MethodPost)
	// swagger:operation POST /libpod/pods/{name}/stop pods PodStopLibpod
	// ---
	// summary: Stop a pod
	// produces:
	// - application/json
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the pod
	//  - in: query
	//    name: t
	//    type: integer
	//    description: timeout
	// responses:
	//   200:
	//     $ref: '#/responses/PodStopReport'
	//   304:
	//     $ref: "#/responses/PodAlreadyStoppedError"
	//   400:
	//     $ref: "#/responses/BadParamError"
	//   404:
	//     $ref: "#/responses/NoSuchPod"
	//   409:
	//     $ref: "#/responses/PodStopReport"
	//   500:
	//     $ref: "#/responses/InternalError"
	r.Handle(VersionedPath("/libpod/pods/{name}/stop"), s.APIHandler(libpod.PodStop)).Methods(http.MethodPost)
	// swagger:operation POST /libpod/pods/{name}/unpause pods PodUnpauseLibpod
	// ---
	// summary: Unpause a pod
	// produces:
	// - application/json
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: the name or ID of the pod
	// responses:
	//   200:
	//     $ref: '#/responses/PodUnpauseReport'
	//   404:
	//     $ref: "#/responses/NoSuchPod"
	//   409:
	//     $ref: '#/responses/PodUnpauseReport'
	//   500:
	//     $ref: "#/responses/InternalError"
	r.Handle(VersionedPath("/libpod/pods/{name}/unpause"), s.APIHandler(libpod.PodUnpause)).Methods(http.MethodPost)
	// swagger:operation GET /libpod/pods/{name}/top pods PodTopLibpod
	// ---
	// summary: List processes
	// description: List processes running inside a pod
	// produces:
	// - application/json
	// parameters:
	//  - in: path
	//    name: name
	//    type: string
	//    required: true
	//    description: Name of pod to query for processes
	//  - in: query
	//    name: stream
	//    type: boolean
	//    description: when true, repeatedly stream the latest output (As of version 4.0)
	//  - in: query
	//    name: delay
	//    type: integer
	//    description: if streaming, delay in seconds between updates. Must be >1. (As of version 4.0)
	//    default: 5
	//  - in: query
	//    name: ps_args
	//    type: string
	//    default: -ef
	//    description: |
	//      arguments to pass to ps such as aux.
	//      Requires ps(1) to be installed in the container if no ps(1) compatible AIX descriptors are used.
	// responses:
	//   200:
	//     $ref: "#/responses/DocsPodTopResponse"
	//   404:
	//     $ref: "#/responses/NoSuchPod"
	//   500:
	//     $ref: "#/responses/InternalError"
	r.Handle(VersionedPath("/libpod/pods/{name}/top"), s.APIHandler(libpod.PodTop)).Methods(http.MethodGet)
	// swagger:operation GET /libpod/pods/stats pods PodStatsAllLibpod
	// ---
	// tags:
	//  - pods
	// summary: Get stats for one or more pods
	// description: Display a live stream of resource usage statistics for the containers in one or more pods
	// parameters:
	//  - in: query
	//    name: all
	//    description: Provide statistics for all running pods.
	//    type: boolean
	//  - in: query
	//    name: namesOrIDs
	//    description: Names or IDs of pods.
	//    type: array
	//    items:
	//      type: string
	// produces:
	// - application/json
	// responses:
	//   200:
	//     $ref: "#/responses/DocsPodTopResponse"
	//   404:
	//     $ref: "#/responses/NoSuchPod"
	//   500:
	//     $ref: "#/responses/InternalError"
	r.Handle(VersionedPath("/libpod/pods/stats"), s.APIHandler(libpod.PodStats)).Methods(http.MethodGet)
	return nil
}
