package libpod

import (
	"fmt"
	"net/http"

	"github.com/containers/podman/v4/libpod"
	"github.com/containers/podman/v4/pkg/api/handlers/utils"
	api "github.com/containers/podman/v4/pkg/api/types"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/domain/infra/abi"
	"github.com/gorilla/schema"
)

func CreateSecret(w http.ResponseWriter, r *http.Request) {
	var (
		runtime = r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
		decoder = r.Context().Value(api.DecoderKey).(*schema.Decoder)
	)

	query := struct {
		Name       string            `schema:"name"`
		Driver     string            `schema:"driver"`
		DriverOpts map[string]string `schema:"driveropts"`
		Labels     map[string]string `schema:"labels"`
	}{
		// override any golang type defaults
	}
	opts := entities.SecretCreateOptions{}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	opts.Driver = query.Driver
	opts.DriverOpts = query.DriverOpts
	opts.Labels = query.Labels

	ic := abi.ContainerEngine{Libpod: runtime}
	report, err := ic.SecretCreate(r.Context(), query.Name, r.Body, opts)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, report)
}
