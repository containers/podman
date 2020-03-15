package server

import (
	"net/http"
	"os"

	"github.com/gorilla/mux"
)

// DefaultPodmanSwaggerSpec provides the default path to the podman swagger spec file
const DefaultPodmanSwaggerSpec = "/usr/share/containers/podman/swagger.yaml"

// RegisterSwaggerHandlers maps the swagger endpoint for the server
func (s *APIServer) RegisterSwaggerHandlers(r *mux.Router) error {
	// This handler does _*NOT*_ provide an UI rather just a swagger spec that an UI could render
	r.PathPrefix("/swagger/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := DefaultPodmanSwaggerSpec
		if p, found := os.LookupEnv("PODMAN_SWAGGER_SPEC"); found {
			path = p
		}
		w.Header().Set("Content-Type", "text/yaml")

		http.ServeFile(w, r, path)
	})
	return nil
}
