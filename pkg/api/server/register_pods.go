package server

import (
	"github.com/containers/libpod/pkg/api/handlers"
	"github.com/gorilla/mux"
)

func (s *APIServer) registerPodsHandlers(r *mux.Router) error {
	r.Handle(VersionedPath("/libpod/pods/"), APIHandler(s.Context, handlers.Pods))
	r.Handle(VersionedPath("/libpod/pods/create"), APIHandler(s.Context, handlers.PodCreate))
	r.Handle(VersionedPath("/libpod/pods/prune"), APIHandler(s.Context, handlers.PodPrune))
	r.Handle(VersionedPath("/libpod/pods/{name:..*}"), APIHandler(s.Context, handlers.PodDelete)).Methods("DELETE")
	r.Handle(VersionedPath("/libpod/pods/{name:..*}"), APIHandler(s.Context, handlers.PodInspect)).Methods("GET")
	r.Handle(VersionedPath("/libpod/pods/{name:..*}/exists"), APIHandler(s.Context, handlers.PodExists))
	r.Handle(VersionedPath("/libpod/pods/{name:..*}/kill"), APIHandler(s.Context, handlers.PodKill))
	r.Handle(VersionedPath("/libpod/pods/{name:..*}/pause"), APIHandler(s.Context, handlers.PodPause))
	r.Handle(VersionedPath("/libpod/pods/{name:..*}/restart"), APIHandler(s.Context, handlers.PodRestart))
	r.Handle(VersionedPath("/libpod/pods/{name:..*}/start"), APIHandler(s.Context, handlers.PodStart))
	r.Handle(VersionedPath("/libpod/pods/{name:..*}/stop"), APIHandler(s.Context, handlers.PodStop))
	r.Handle(VersionedPath("/libpod/pods/{name:..*}/unpause"), APIHandler(s.Context, handlers.PodUnpause))
	return nil
}
