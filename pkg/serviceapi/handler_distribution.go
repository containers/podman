package serviceapi

import (
	"github.com/gorilla/mux"
)

func (s *APIServer) registerDistributionHandlers(r *mux.Router) error {
	r.HandleFunc(versionedPath("/distribution/{name:..*}/json"), s.unsupportedHandler)
	return nil
}
