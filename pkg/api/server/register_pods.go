package server

import (
	"net/http"

	"github.com/containers/libpod/pkg/api/handlers/libpod"
	"github.com/gorilla/mux"
)

func (s *APIServer) registerPodsHandlers(r *mux.Router) error {
	// swagger:operation GET /libpod/pods/json libpod libpodPods
	// ---
	// summary: List pods
	// produces:
	// - application/json
	// parameters:
	// - in: query
	//   name: filters
	//   descriptions: needs description and plumbing for filters
	// responses:
	//   '200':
	//       properties:
	//        items:
	//          $ref: "#/responses/ListPodsResponse"
	//        type: array
	//   '400':
	//      $ref: "#/responses/BadParamError"
	//   '500':
	//      $ref: "#/responses/InternalError"
	r.Handle(VersionedPath("/libpod/pods/json"), APIHandler(s.Context, libpod.Pods)).Methods(http.MethodGet)
	// swagger:operation POST /libpod/pods/create libpod libpodPodCreate
	r.Handle(VersionedPath("/libpod/pods/create"), APIHandler(s.Context, libpod.PodCreate)).Methods(http.MethodPost)
	// swagger:operation POST /libpod/pods/prune libpod libpodPodPrune
	// ---
	// summary: Prune unused pods
	// parameters:
	//  - in: query
	//    name: force
	//    description: force delete
	//    type: bool
	//    default: false
	// produces:
	// - application/json
	// responses:
	//   '204':
	//      description: no error
	//   '400':
	//      $ref: "#/responses/BadParamError"
	//   '500':
	//      $ref: "#/responses/InternalError"
	r.Handle(VersionedPath("/libpod/pods/prune"), APIHandler(s.Context, libpod.PodPrune)).Methods(http.MethodPost)
	// swagger:operation DELETE /libpod/pods/{nameOrID} libpod libpodPodDelete
	// ---
	// summary: Remove pod
	// produces:
	// - application/json
	// parameters:
	//  - in: path
	//    name: nameOrID
	//    required: true
	//    description: the name or ID of the pod
	//  - in: query
	//    name: force
	//    type: bool
	//    description: force delete
	// responses:
	//   '204':
	//      description: no error
	//   '400':
	//      $ref: "#/responses/BadParamError"
	//   '404':
	//      $ref: "#/responses/NoSuchPod"
	//   '500':
	//      $ref: "#/responses/InternalError"
	r.Handle(VersionedPath("/libpod/pods/{name:..*}"), APIHandler(s.Context, libpod.PodDelete)).Methods(http.MethodDelete)
	// swagger:operation GET /libpod/pods/{nameOrID}/json libpod libpodPodInspect
	// ---
	// summary: Inspect pod
	// produces:
	// - application/json
	// parameters:
	//  - in: path
	//    name: nameOrID
	//    required: true
	//    description: the name or ID of the pod
	// responses:
	//   '200':
	//      $ref: "#/responses/InspectPodResponse"
	//   '404':
	//      $ref: "#/responses/NoSuchPod"
	//   '500':
	//      $ref: "#/responses/InternalError"
	r.Handle(VersionedPath("/libpod/pods/{name:..*}/json"), APIHandler(s.Context, libpod.PodInspect)).Methods(http.MethodGet)
	// swagger:operation GET /libpod/pods/{nameOrID}/exists libpod libpodPodExists
	// ---
	// summary: Pod exists
	// description: Check if a pod exists by name or ID
	// produces:
	// - application/json
	// parameters:
	//  - in: path
	//    name: nameOrID
	//    required: true
	//    description: the name or ID of the pod
	// responses:
	//   '204':
	//      description: pod exists
	//   '404':
	//      $ref: "#/responses/NoSuchPod"
	//   '500':
	//      $ref: "#/responses/InternalError"
	r.Handle(VersionedPath("/libpod/pods/{name:..*}/exists"), APIHandler(s.Context, libpod.PodExists)).Methods(http.MethodGet)
	// swagger:operation POST /libpod/pods/{nameOrID}/kill libpod libpodPodKill
	// ---
	// summary: Kill a pod
	// produces:
	// - application/json
	// parameters:
	//  - in: path
	//    name: nameOrID
	//    required: true
	//    description: the name or ID of the pod
	//  - in: query
	//    name: signal
	//    type: int
	//    description: signal to be sent to pod
	// responses:
	//   '204':
	//      description: no error
	//   '400':
	//      $ref: "#/responses/BadParamError"
	//   '404':
	//      $ref: "#/responses/NoSuchPod"
	//   '409':
	//      $ref: "#/responses/ConflictError"
	//   '500':
	//      $ref: "#/responses/InternalError"
	r.Handle(VersionedPath("/libpod/pods/{name:..*}/kill"), APIHandler(s.Context, libpod.PodKill)).Methods(http.MethodPost)
	// swagger:operation POST /libpod/pods/{nameOrID}/pause libpod libpodPodPause
	// ---
	// summary: Pause a pod
	// produces:
	// - application/json
	// parameters:
	//  - in: path
	//    name: nameOrID
	//    required: true
	//    description: the name or ID of the pod
	// responses:
	//   '204':
	//      description: no error
	//   '404':
	//      $ref: "#/responses/NoSuchPod"
	//   '500':
	//      $ref: "#/responses/InternalError"
	r.Handle(VersionedPath("/libpod/pods/{name:..*}/pause"), APIHandler(s.Context, libpod.PodPause)).Methods(http.MethodPost)
	// swagger:operation POST /libpod/pods/{nameOrID}/restart libpod libpodPodRestart
	// ---
	// summary: Restart a pod
	// produces:
	// - application/json
	// parameters:
	//  - in: path
	//    name: nameOrID
	//    required: true
	//    description: the name or ID of the pod
	// responses:
	//   '204':
	//      description: no error
	//   '404':
	//      $ref: "#/responses/NoSuchPod"
	//   '500':
	//      $ref: "#/responses/InternalError"
	r.Handle(VersionedPath("/libpod/pods/{name:..*}/restart"), APIHandler(s.Context, libpod.PodRestart)).Methods(http.MethodPost)
	// swagger:operation POST /libpod/pods/{nameOrID}/start libpod libpodPodStart
	// ---
	// summary: Start a pod
	// produces:
	// - application/json
	// parameters:
	//  - in: path
	//    name: nameOrID
	//    required: true
	//    description: the name or ID of the pod
	// responses:
	//   '204':
	//      description: no error
	//   '304':
	//      $ref: "#/responses/PodAlreadyStartedError"
	//   '404':
	//      $ref: "#/responses/NoSuchPod"
	//   '500':
	//      $ref: "#/responses/InternalError"
	r.Handle(VersionedPath("/libpod/pods/{name:..*}/start"), APIHandler(s.Context, libpod.PodStart)).Methods(http.MethodPost)
	// swagger:operation POST /libpod/pods/{nameOrID}/stop libpod libpodPodStop
	// ---
	// summary: Stop a pod
	// produces:
	// - application/json
	// parameters:
	//  - in: path
	//    name: nameOrID
	//    required: true
	//    description: the name or ID of the pod
	//  - in: query
	//    name: t
	//    type: int
	//    description: timeout
	// responses:
	//   '204':
	//      description: no error
	//   '304':
	//      $ref: "#/responses/PodAlreadyStoppedError"
	//   '400':
	//      $ref: "#/responses/BadParamError"
	//   '404':
	//      $ref: "#/responses/NoSuchPod"
	//   '500':
	//      $ref: "#/responses/InternalError"
	r.Handle(VersionedPath("/libpod/pods/{name:..*}/stop"), APIHandler(s.Context, libpod.PodStop)).Methods(http.MethodPost)
	// swagger:operation POST /libpod/pods/{nameOrID}/unpause libpod libpodPodUnpause
	// ---
	// summary: Unpause a pod
	// produces:
	// - application/json
	// parameters:
	//  - in: path
	//    name: nameOrID
	//    required: true
	//    description: the name or ID of the pod
	// responses:
	//   '204':
	//      description: no error
	//   '404':
	//      $ref: "#/responses/NoSuchPod"
	//   '500':
	//      $ref: "#/responses/InternalError"
	r.Handle(VersionedPath("/libpod/pods/{name:..*}/unpause"), APIHandler(s.Context, libpod.PodUnpause)).Methods(http.MethodPost)
	return nil
}
