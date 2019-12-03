package server

import (
	"github.com/containers/libpod/pkg/api/handlers"
	"github.com/gorilla/mux"
)

func (s *APIServer) RegisterContainersHandlers(r *mux.Router) error {
	r.HandleFunc(VersionedPath("/containers/create"), APIHandler(s.Context, handlers.CreateContainer)).Methods("POST")
	r.HandleFunc(VersionedPath("/containers/json"), APIHandler(s.Context, handlers.ListContainers)).Methods("GET")
	r.HandleFunc(VersionedPath("/containers/prune"), APIHandler(s.Context, handlers.PruneContainers)).Methods("POST")
	r.HandleFunc(VersionedPath("/containers/{name:..*}"), APIHandler(s.Context, handlers.RemoveContainer)).Methods("DELETE")
	r.HandleFunc(VersionedPath("/containers/{name:..*}/json"), APIHandler(s.Context, handlers.GetContainer)).Methods("GET")
	r.HandleFunc(VersionedPath("/containers/{name:..*}/kill"), APIHandler(s.Context, handlers.KillContainer)).Methods("POST")
	r.HandleFunc(VersionedPath("/containers/{name:..*}/logs"), APIHandler(s.Context, handlers.LogsFromContainer)).Methods("GET")
	r.HandleFunc(VersionedPath("/containers/{name:..*}/pause"), APIHandler(s.Context, handlers.PauseContainer)).Methods("POST")
	r.HandleFunc(VersionedPath("/containers/{name:..*}/rename"), APIHandler(s.Context, handlers.UnsupportedHandler)).Methods("POST")
	r.HandleFunc(VersionedPath("/containers/{name:..*}/restart"), APIHandler(s.Context, handlers.RestartContainer)).Methods("POST")
	r.HandleFunc(VersionedPath("/containers/{name:..*}/start"), APIHandler(s.Context, handlers.StartContainer)).Methods("POST")
	r.HandleFunc(VersionedPath("/containers/{name:..*}/stats"), APIHandler(s.Context, handlers.StatsContainer)).Methods("GET")
	r.HandleFunc(VersionedPath("/containers/{name:..*}/stop"), APIHandler(s.Context, handlers.StopContainer)).Methods("POST")
	r.HandleFunc(VersionedPath("/containers/{name:..*}/top"), APIHandler(s.Context, handlers.TopContainer)).Methods("GET")
	r.HandleFunc(VersionedPath("/containers/{name:..*}/unpause"), APIHandler(s.Context, handlers.UnpauseContainer)).Methods("POST")
	r.HandleFunc(VersionedPath("/containers/{name:..*}/wait"), APIHandler(s.Context, handlers.WaitContainer)).Methods("POST")

	// libpod endpoints
	r.HandleFunc(VersionedPath("/libpod/containers/{name:..*}/exists"), APIHandler(s.Context, handlers.ContainerExists))
	return nil
}
