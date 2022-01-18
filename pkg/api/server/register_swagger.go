package server

import (
	"net/http"

	"github.com/containers/podman/v4/pkg/api/handlers/libpod"
	"github.com/gorilla/mux"
)

// registerSwaggerHandlers maps the swagger endpoint for the server
func (s *APIServer) registerSwaggerHandlers(r *mux.Router) error {
	// This handler does _*NOT*_ provide an UI rather just a swagger spec that an UI could render
	r.HandleFunc(VersionedPath("/libpod/swagger"), s.APIHandler(libpod.ServeSwagger)).Methods(http.MethodGet)
	return nil
}
