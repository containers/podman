package server

import (
	"github.com/containers/libpod/pkg/api/handlers"

	"github.com/gorilla/mux"
)

func (s *APIServer) registerImagesHandlers(r *mux.Router) error {
	r.Handle(VersionedPath("/images/create"), APIHandler(s.Context, handlers.CreateImageFromImage)).Methods("POST").Queries("fromImage", "{fromImage}")
	r.Handle(VersionedPath("/images/create"), APIHandler(s.Context, handlers.CreateImageFromSrc)).Methods("POST").Queries("fromSrc", "{fromSrc}")
	r.Handle(VersionedPath("/images/json"), APIHandler(s.Context, handlers.GetImages)).Methods("GET")
	r.Handle(VersionedPath("/images/load"), APIHandler(s.Context, handlers.LoadImage)).Methods("POST")
	r.Handle(VersionedPath("/images/prune"), APIHandler(s.Context, handlers.PruneImages)).Methods("POST")
	r.Handle(VersionedPath("/images/{name:..*}"), APIHandler(s.Context, handlers.RemoveImage)).Methods("DELETE")
	r.Handle(VersionedPath("/images/{name:..*}/get"), APIHandler(s.Context, handlers.ExportImage)).Methods("GET")
	r.Handle(VersionedPath("/images/{name:..*}/json"), APIHandler(s.Context, handlers.GetImage))
	r.Handle(VersionedPath("/images/{name:..*}/tag"), APIHandler(s.Context, handlers.TagImage)).Methods("POST")

	// commit has a different endpoint
	r.Handle(VersionedPath("/commit"), APIHandler(s.Context, handlers.CommitContainer)).Methods("POST")
	// libpod
	r.Handle(VersionedPath("/libpod/images/{name:..*}/exists"), APIHandler(s.Context, handlers.ImageExists))

	return nil
}
