package libpod

import (
	"net/http"
	"os"

	"github.com/containers/image/v5/manifest"
	"github.com/containers/podman/v3/libpod/define"
	"github.com/containers/podman/v3/libpod/network/types"
	"github.com/containers/podman/v3/pkg/api/handlers/utils"
	"github.com/containers/podman/v3/pkg/domain/entities"
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
	Body manifest.Schema2List
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

// Prune pod
// swagger:response PodPruneReport
type swagPrunePodResponse struct {
	// in:body
	Body entities.PodPruneReport
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

// Network rm
// swagger:response NetworkRmReport
type swagNetworkRmReport struct {
	// in:body
	Body []entities.NetworkRmReport
}

// Network inspect
// swagger:response NetworkInspectReport
type swagNetworkInspectReport struct {
	// in:body
	Body types.Network
}

// Network list
// swagger:response NetworkListReport
type swagNetworkListReport struct {
	// in:body
	Body []types.Network
}

// Network create
// swagger:model NetworkCreateLibpod
type swagNetworkCreateLibpod struct {
	types.Network
}

// Network create
// swagger:response NetworkCreateReport
type swagNetworkCreateReport struct {
	// in:body
	Body types.Network
}

// Network prune
// swagger:response NetworkPruneResponse
type swagNetworkPruneResponse struct {
	// in:body
	Body []entities.NetworkPruneReport
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
