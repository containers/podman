package server

import (
	"net/http"

	"github.com/containers/libpod/pkg/api/handlers/libpod"
	"github.com/gorilla/mux"
)

func (s *APIServer) registerPodsHandlers(r *mux.Router) error {
	r.Handle(VersionedPath("/libpod/pods/json"), APIHandler(s.Context, libpod.Pods)).Methods(http.MethodGet)
	r.Handle(VersionedPath("/libpod/pods/create"), APIHandler(s.Context, libpod.PodCreate)).Methods(http.MethodPost)
	r.Handle(VersionedPath("/libpod/pods/prune"), APIHandler(s.Context, libpod.PodPrune)).Methods(http.MethodPost)
	r.Handle(VersionedPath("/libpod/pods/{name:..*}"), APIHandler(s.Context, libpod.PodDelete)).Methods(http.MethodDelete)
	r.Handle(VersionedPath("/libpod/pods/{name:..*}"), APIHandler(s.Context, libpod.PodInspect)).Methods(http.MethodGet)
	r.Handle(VersionedPath("/libpod/pods/{name:..*}/exists"), APIHandler(s.Context, libpod.PodExists)).Methods(http.MethodGet)
	r.Handle(VersionedPath("/libpod/pods/{name:..*}/kill"), APIHandler(s.Context, libpod.PodKill)).Methods(http.MethodPost)
	r.Handle(VersionedPath("/libpod/pods/{name:..*}/pause"), APIHandler(s.Context, libpod.PodPause)).Methods(http.MethodPost)
	r.Handle(VersionedPath("/libpod/pods/{name:..*}/restart"), APIHandler(s.Context, libpod.PodRestart)).Methods(http.MethodPost)
	r.Handle(VersionedPath("/libpod/pods/{name:..*}/start"), APIHandler(s.Context, libpod.PodStart)).Methods(http.MethodPost)
	r.Handle(VersionedPath("/libpod/pods/{name:..*}/stop"), APIHandler(s.Context, libpod.PodStop)).Methods(http.MethodPost)
	r.Handle(VersionedPath("/libpod/pods/{name:..*}/unpause"), APIHandler(s.Context, libpod.PodUnpause)).Methods(http.MethodPost)
	return nil
}
