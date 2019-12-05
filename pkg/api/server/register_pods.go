package server

import (
	"github.com/containers/libpod/pkg/api/handlers/libpod"
	"github.com/gorilla/mux"
)

func (s *APIServer) registerPodsHandlers(r *mux.Router) error {
	r.Handle(VersionedPath("/libpod/pods/"), APIHandler(s.Context, libpod.Pods))
	r.Handle(VersionedPath("/libpod/pods/create"), APIHandler(s.Context, libpod.PodCreate))
	r.Handle(VersionedPath("/libpod/pods/prune"), APIHandler(s.Context, libpod.PodPrune))
	r.Handle(VersionedPath("/libpod/pods/{name:..*}"), APIHandler(s.Context, libpod.PodDelete)).Methods("DELETE")
	r.Handle(VersionedPath("/libpod/pods/{name:..*}"), APIHandler(s.Context, libpod.PodInspect)).Methods("GET")
	r.Handle(VersionedPath("/libpod/pods/{name:..*}/exists"), APIHandler(s.Context, libpod.PodExists))
	r.Handle(VersionedPath("/libpod/pods/{name:..*}/kill"), APIHandler(s.Context, libpod.PodKill))
	r.Handle(VersionedPath("/libpod/pods/{name:..*}/pause"), APIHandler(s.Context, libpod.PodPause))
	r.Handle(VersionedPath("/libpod/pods/{name:..*}/restart"), APIHandler(s.Context, libpod.PodRestart))
	r.Handle(VersionedPath("/libpod/pods/{name:..*}/start"), APIHandler(s.Context, libpod.PodStart))
	r.Handle(VersionedPath("/libpod/pods/{name:..*}/stop"), APIHandler(s.Context, libpod.PodStop))
	r.Handle(VersionedPath("/libpod/pods/{name:..*}/unpause"), APIHandler(s.Context, libpod.PodUnpause))
	return nil
}
