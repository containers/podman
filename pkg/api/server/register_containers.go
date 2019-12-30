package server

import (
	"net/http"

	"github.com/containers/libpod/pkg/api/handlers"
	"github.com/containers/libpod/pkg/api/handlers/generic"
	"github.com/containers/libpod/pkg/api/handlers/libpod"
	"github.com/gorilla/mux"
)

func (s *APIServer) RegisterContainersHandlers(r *mux.Router) error {
	r.HandleFunc(VersionedPath("/containers/create"), APIHandler(s.Context, generic.CreateContainer)).Methods(http.MethodPost)
	r.HandleFunc(VersionedPath("/containers/json"), APIHandler(s.Context, generic.ListContainers)).Methods(http.MethodGet)
	r.HandleFunc(VersionedPath("/containers/prune"), APIHandler(s.Context, generic.PruneContainers)).Methods(http.MethodPost)
	r.HandleFunc(VersionedPath("/containers/{name:..*}"), APIHandler(s.Context, generic.RemoveContainer)).Methods(http.MethodDelete)
	r.HandleFunc(VersionedPath("/containers/{name:..*}/json"), APIHandler(s.Context, generic.GetContainer)).Methods(http.MethodGet)
	r.HandleFunc(VersionedPath("/containers/{name:..*}/kill"), APIHandler(s.Context, generic.KillContainer)).Methods(http.MethodPost)
	r.HandleFunc(VersionedPath("/containers/{name:..*}/logs"), APIHandler(s.Context, generic.LogsFromContainer)).Methods(http.MethodGet)
	r.HandleFunc(VersionedPath("/containers/{name:..*}/pause"), APIHandler(s.Context, handlers.PauseContainer)).Methods(http.MethodPost)
	r.HandleFunc(VersionedPath("/containers/{name:..*}/rename"), APIHandler(s.Context, handlers.UnsupportedHandler)).Methods(http.MethodPost)
	r.HandleFunc(VersionedPath("/containers/{name:..*}/restart"), APIHandler(s.Context, handlers.RestartContainer)).Methods(http.MethodPost)
	r.HandleFunc(VersionedPath("/containers/{name:..*}/start"), APIHandler(s.Context, handlers.StartContainer)).Methods(http.MethodPost)
	r.HandleFunc(VersionedPath("/containers/{name:..*}/stats"), APIHandler(s.Context, generic.StatsContainer)).Methods(http.MethodGet)
	r.HandleFunc(VersionedPath("/containers/{name:..*}/stop"), APIHandler(s.Context, handlers.StopContainer)).Methods(http.MethodPost)
	r.HandleFunc(VersionedPath("/containers/{name:..*}/top"), APIHandler(s.Context, handlers.TopContainer)).Methods(http.MethodGet)
	r.HandleFunc(VersionedPath("/containers/{name:..*}/unpause"), APIHandler(s.Context, handlers.UnpauseContainer)).Methods(http.MethodPost)
	r.HandleFunc(VersionedPath("/containers/{name:..*}/wait"), APIHandler(s.Context, generic.WaitContainer)).Methods(http.MethodPost)

	// libpod endpoints
	r.HandleFunc(VersionedPath("/libpod/containers/create"), APIHandler(s.Context, libpod.CreateContainer)).Methods(http.MethodPost)
	r.HandleFunc(VersionedPath("/libpod/containers/json"), APIHandler(s.Context, libpod.ListContainers)).Methods(http.MethodGet)
	r.HandleFunc(VersionedPath("/libpod/containers/prune"), APIHandler(s.Context, libpod.PruneContainers)).Methods(http.MethodPost)
	r.HandleFunc(VersionedPath("/libpod/containers/showmounted"), APIHandler(s.Context, libpod.ShowMountedContainers)).Methods(http.MethodGet)
	r.HandleFunc(VersionedPath("/libpod/containers/{name:..*}"), APIHandler(s.Context, libpod.RemoveContainer)).Methods(http.MethodDelete)
	r.HandleFunc(VersionedPath("/libpod/containers/{name:..*}/json"), APIHandler(s.Context, libpod.GetContainer)).Methods(http.MethodGet)
	r.HandleFunc(VersionedPath("/libpod/containers/{name:..*}/kill"), APIHandler(s.Context, libpod.MountContainer)).Methods(http.MethodGet)
	r.HandleFunc(VersionedPath("/libpod/containers/{name:..*}/mount"), APIHandler(s.Context, libpod.LogsFromContainer)).Methods(http.MethodGet)
	r.HandleFunc(VersionedPath("/libpod/containers/{name:..*}/logs"), APIHandler(s.Context, libpod.LogsFromContainer)).Methods(http.MethodGet)
	r.HandleFunc(VersionedPath("/libpod/containers/{name:..*}/pause"), APIHandler(s.Context, handlers.PauseContainer)).Methods(http.MethodPost)
	r.HandleFunc(VersionedPath("/libpod/containers/{name:..*}/rename"), APIHandler(s.Context, handlers.UnsupportedHandler)).Methods(http.MethodPost)
	r.HandleFunc(VersionedPath("/libpod/containers/{name:..*}/restart"), APIHandler(s.Context, handlers.RestartContainer)).Methods(http.MethodPost)
	r.HandleFunc(VersionedPath("/libpod/containers/{name:..*}/start"), APIHandler(s.Context, handlers.StartContainer)).Methods(http.MethodPost)
	r.HandleFunc(VersionedPath("/libpod/containers/{name:..*}/stats"), APIHandler(s.Context, libpod.StatsContainer)).Methods(http.MethodGet)
	r.HandleFunc(VersionedPath("/libpod/containers/{name:..*}/top"), APIHandler(s.Context, handlers.TopContainer)).Methods(http.MethodGet)
	r.HandleFunc(VersionedPath("/libpod/containers/{name:..*}/unpause"), APIHandler(s.Context, handlers.UnpauseContainer)).Methods(http.MethodPost)
	r.HandleFunc(VersionedPath("/libpod/containers/{name:..*}/wait"), APIHandler(s.Context, libpod.WaitContainer)).Methods(http.MethodPost)
	r.HandleFunc(VersionedPath("/libpod/containers/{name:..*}/exists"), APIHandler(s.Context, libpod.ContainerExists)).Methods(http.MethodGet)
	r.HandleFunc(VersionedPath("/libpod/containers/{name:..*}/stop"), APIHandler(s.Context, handlers.StopContainer)).Methods(http.MethodPost)
	return nil
}
