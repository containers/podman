package libpod

import (
	"net/http"
	"os"

	"github.com/containers/podman/v4/pkg/api/handlers/utils"
	"github.com/pkg/errors"
)

// DefaultPodmanSwaggerSpec provides the default path to the podman swagger spec file
const DefaultPodmanSwaggerSpec = "/usr/share/containers/podman/swagger.yaml"

func ServeSwagger(w http.ResponseWriter, r *http.Request) {
	path := DefaultPodmanSwaggerSpec
	if p, found := os.LookupEnv("PODMAN_SWAGGER_SPEC"); found {
		path = p
	}
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			utils.InternalServerError(w, errors.Errorf("swagger spec %q does not exist", path))
			return
		}
		utils.InternalServerError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/yaml")
	http.ServeFile(w, r, path)
}
