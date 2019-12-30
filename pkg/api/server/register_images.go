package server

import (
	"net/http"

	"github.com/containers/libpod/pkg/api/handlers"
	"github.com/containers/libpod/pkg/api/handlers/generic"
	"github.com/containers/libpod/pkg/api/handlers/libpod"
	"github.com/gorilla/mux"
)

func (s *APIServer) registerImagesHandlers(r *mux.Router) error {
	r.Handle(VersionedPath("/images/create"), APIHandler(s.Context, generic.CreateImageFromImage)).Methods(http.MethodPost).Queries("fromImage", "{fromImage}")
	r.Handle(VersionedPath("/images/create"), APIHandler(s.Context, generic.CreateImageFromSrc)).Methods(http.MethodPost).Queries("fromSrc", "{fromSrc}")
	r.Handle(VersionedPath("/images/history"), APIHandler(s.Context, handlers.HistoryImage)).Methods(http.MethodGet)
	r.Handle(VersionedPath("/images/json"), APIHandler(s.Context, generic.GetImages)).Methods(http.MethodGet)
	r.Handle(VersionedPath("/images/load"), APIHandler(s.Context, handlers.LoadImage)).Methods(http.MethodPost)
	r.Handle(VersionedPath("/images/prune"), APIHandler(s.Context, generic.PruneImages)).Methods(http.MethodPost)
	r.Handle(VersionedPath("/images/search"), APIHandler(s.Context, handlers.SearchImages)).Methods(http.MethodGet)
	r.Handle(VersionedPath("/images/{name:..*}"), APIHandler(s.Context, handlers.RemoveImage)).Methods(http.MethodDelete)
	r.Handle(VersionedPath("/images/{name:..*}/get"), APIHandler(s.Context, generic.ExportImage)).Methods(http.MethodGet)
	r.Handle(VersionedPath("/images/{name:..*}/json"), APIHandler(s.Context, generic.GetImage))
	r.Handle(VersionedPath("/images/{name:..*}/tag"), APIHandler(s.Context, handlers.TagImage)).Methods(http.MethodPost)

	// commit has a different endpoint
	r.Handle(VersionedPath("/commit"), APIHandler(s.Context, generic.CommitContainer)).Methods(http.MethodPost)

	// libpod
	r.Handle(VersionedPath("/libpod/images/{name:..*}/exists"), APIHandler(s.Context, libpod.ImageExists))
	r.Handle(VersionedPath("/libpod/images/{name:..*}/tree"), APIHandler(s.Context, libpod.ImageTree))
	r.Handle(VersionedPath("/libpod/images/history"), APIHandler(s.Context, handlers.HistoryImage)).Methods(http.MethodGet)
	r.Handle(VersionedPath("/libpod/images/json"), APIHandler(s.Context, libpod.GetImages)).Methods(http.MethodGet)
	r.Handle(VersionedPath("/libpod/images/load"), APIHandler(s.Context, handlers.LoadImage)).Methods(http.MethodPost)
	r.Handle(VersionedPath("/libpod/images/prune"), APIHandler(s.Context, libpod.PruneImages)).Methods(http.MethodPost)
	r.Handle(VersionedPath("/libpod/images/search"), APIHandler(s.Context, handlers.SearchImages)).Methods(http.MethodGet)
	r.Handle(VersionedPath("/libpod/images/{name:..*}"), APIHandler(s.Context, handlers.RemoveImage)).Methods(http.MethodDelete)
	r.Handle(VersionedPath("/libpod/images/{name:..*}/get"), APIHandler(s.Context, libpod.ExportImage)).Methods(http.MethodGet)
	r.Handle(VersionedPath("/libpod/images/{name:..*}/json"), APIHandler(s.Context, libpod.GetImage))
	r.Handle(VersionedPath("/libpod/images/{name:..*}/tag"), APIHandler(s.Context, handlers.TagImage)).Methods(http.MethodPost)

	r.Handle(VersionedPath("/build"), APIHandler(s.Context, handlers.BuildImage)).Methods(http.MethodPost)
	return nil
}
