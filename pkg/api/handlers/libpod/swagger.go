package libpod

import (
	"net/http"
	"os"

	"github.com/containers/image/v5/manifest"
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/api/handlers/utils"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/pkg/errors"
)

// DefaultPodmanSwaggerSpec provides the default path to the podman swagger spec file
const DefaultPodmanSwaggerSpec = "/usr/share/containers/podman/swagger.yaml"

// List Containers
// swagger:response ListContainers
type swagInspectPodResponse struct {
	// in:body
	Body []entities.ListContainer
}

// Inspect Manifest
// swagger:response InspectManifest
type swagInspectManifestResponse struct {
	// in:body
	Body manifest.List
}

// Kill Pod
// swagger:response PodKillReport
type swagKillPodResponse struct {
	// in:body
	Body entities.PodKillReport
}

// Pause pod
// swagger:response PodPauseReport
type swagPausePodResponse struct {
	// in:body
	Body entities.PodPauseReport
}

// Unpause pod
// swagger:response PodUnpauseReport
type swagUnpausePodResponse struct {
	// in:body
	Body entities.PodUnpauseReport
}

// Stop pod
// swagger:response PodStopReport
type swagStopPodResponse struct {
	// in:body
	Body entities.PodStopReport
}

// Restart pod
// swagger:response PodRestartReport
type swagRestartPodResponse struct {
	// in:body
	Body entities.PodRestartReport
}

// Start pod
// swagger:response PodStartReport
type swagStartPodResponse struct {
	// in:body
	Body entities.PodStartReport
}

// Rm pod
// swagger:response PodRmReport
type swagRmPodResponse struct {
	// in:body
	Body entities.PodRmReport
}

// Info
// swagger:response InfoResponse
type swagInfoResponse struct {
	// in:body
	Body define.Info
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
