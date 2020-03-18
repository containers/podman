package libpod

import (
	"net/http"
	"os"

	"github.com/containers/image/v5/manifest"
	"github.com/containers/libpod/pkg/api/handlers/utils"
	"github.com/pkg/errors"
)

// DefaultPodmanSwaggerSpec provides the default path to the podman swagger spec file
const DefaultPodmanSwaggerSpec = "/usr/share/containers/podman/swagger.yaml"

// List Containers
// swagger:response ListContainers
type swagInspectPodResponse struct {
	// in:body
	Body []ListContainer
}

// Inspect Manifest
// swagger:response InspectManifest
type swagInspectManifestResponse struct {
	// in:body
	Body manifest.List
}

func ServeSwagger(w http.ResponseWriter, r *http.Request) {
	path := DefaultPodmanSwaggerSpec
	if p, found := os.LookupEnv("PODMAN_SWAGGER_SPEC"); found {
		path = p
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			utils.InternalServerError(w, errors.Errorf("file %q does not exist", path))
			return
		}
		utils.InternalServerError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/yaml")
	http.ServeFile(w, r, path)
}
