package server

import (
	"github.com/containers/libpod/pkg/api/handlers"
	"github.com/containers/libpod/pkg/api/handlers/generic"
	"github.com/containers/libpod/pkg/api/handlers/libpod"

	"github.com/gorilla/mux"
)

func (s *APIServer) registerImagesHandlers(r *mux.Router) error {
	r.Handle(VersionedPath("/images/create"), APIHandler(s.Context, generic.CreateImageFromImage)).Methods("POST").Queries("fromImage", "{fromImage}")
	r.Handle(VersionedPath("/images/create"), APIHandler(s.Context, generic.CreateImageFromSrc)).Methods("POST").Queries("fromSrc", "{fromSrc}")
	r.Handle(VersionedPath("/images/history"), APIHandler(s.Context, handlers.HistoryImage)).Methods("GET")
	r.Handle(VersionedPath("/images/json"), APIHandler(s.Context, generic.GetImages)).Methods("GET")
	r.Handle(VersionedPath("/images/load"), APIHandler(s.Context, generic.LoadImage)).Methods("POST")
	r.Handle(VersionedPath("/images/prune"), APIHandler(s.Context, generic.PruneImages)).Methods("POST")
	r.Handle(VersionedPath("/images/{name:..*}"), APIHandler(s.Context, handlers.RemoveImage)).Methods("DELETE")
	r.Handle(VersionedPath("/images/{name:..*}/get"), APIHandler(s.Context, generic.ExportImage)).Methods("GET")
	r.Handle(VersionedPath("/images/{name:..*}/json"), APIHandler(s.Context, generic.GetImage))
	r.Handle(VersionedPath("/images/{name:..*}/tag"), APIHandler(s.Context, handlers.TagImage)).Methods("POST")

	// commit has a different endpoint
	r.Handle(VersionedPath("/commit"), APIHandler(s.Context, generic.CommitContainer)).Methods("POST")

	// libpod
	r.Handle(VersionedPath("/libpod/images/{name:..*}/exists"), APIHandler(s.Context, libpod.ImageExists))
	r.Handle(VersionedPath("/libpod/images/{name:..*}/tree"), APIHandler(s.Context, libpod.ImageTree))
	r.Handle(VersionedPath("/libpod/images/history"), APIHandler(s.Context, handlers.HistoryImage)).Methods("GET")
	r.Handle(VersionedPath("/libpod/images/json"), APIHandler(s.Context, libpod.GetImages)).Methods("GET")
	r.Handle(VersionedPath("/libpod/images/load"), APIHandler(s.Context, libpod.LoadImage)).Methods("POST")
	r.Handle(VersionedPath("/libpod/images/prune"), APIHandler(s.Context, libpod.PruneImages)).Methods("POST")
	r.Handle(VersionedPath("/libpod/images/{name:..*}"), APIHandler(s.Context, handlers.RemoveImage)).Methods("DELETE")
	r.Handle(VersionedPath("/libpod/images/{name:..*}/get"), APIHandler(s.Context, libpod.ExportImage)).Methods("GET")
	r.Handle(VersionedPath("/libpod/images/{name:..*}/json"), APIHandler(s.Context, libpod.GetImage))
	r.Handle(VersionedPath("/libpod/images/{name:..*}/tag"), APIHandler(s.Context, handlers.TagImage)).Methods("POST")

	r.Handle(VersionedPath("/build"), APIHandler(s.Context, handlers.BuildImage)).Methods("POST")
	return nil
}
