//go:build !remote

package libpod

import (
	"fmt"
	"net/http"

	"github.com/containers/podman/v5/libpod"
	"github.com/containers/podman/v5/pkg/api/handlers/utils"
	api "github.com/containers/podman/v5/pkg/api/types"
	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/containers/podman/v5/pkg/domain/infra/abi"
	"github.com/containers/podman/v5/pkg/errorhandling"
	"github.com/gorilla/schema"
	"go.podman.io/image/v5/types"
)

func AutoUpdate(w http.ResponseWriter, r *http.Request) {
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)

	query := struct {
		Authfile  string `schema:"authfile"`
		DryRun    bool   `schema:"dryRun"`
		Rollback  bool   `schema:"rollback"`
		TLSVerify bool   `schema:"tlsVerify"`
	}{
		TLSVerify: true,
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	containerEngine := abi.ContainerEngine{Libpod: runtime}

	options := entities.AutoUpdateOptions{
		Authfile:              query.Authfile,
		DryRun:                query.DryRun,
		Rollback:              query.Rollback,
		InsecureSkipTLSVerify: types.NewOptionalBool(!query.TLSVerify),
	}

	allReports, failures := containerEngine.AutoUpdate(r.Context(), options)
	if allReports == nil {
		utils.Error(w, http.StatusInternalServerError, errorhandling.JoinErrors(failures))
		return
	}

	utils.WriteResponse(w, http.StatusOK, allReports)
}
