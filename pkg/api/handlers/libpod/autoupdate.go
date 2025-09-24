//go:build !remote

package libpod

import (
	"fmt"
	"net/http"

	"github.com/containers/podman/v5/libpod"
	"github.com/containers/podman/v5/pkg/api/handlers"
	"github.com/containers/podman/v5/pkg/api/handlers/utils"
	api "github.com/containers/podman/v5/pkg/api/types"
	"github.com/containers/podman/v5/pkg/auth"
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
		DryRun    bool `schema:"dryRun"`
		Rollback  bool `schema:"rollback"`
		TLSVerify bool `schema:"tlsVerify"`
	}{
		TLSVerify: true,
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	_, authfile, err := auth.GetCredentials(r)
	if err != nil {
		utils.Error(w, http.StatusBadRequest, err)
		return
	}
	defer auth.RemoveAuthfile(authfile)

	containerEngine := abi.ContainerEngine{Libpod: runtime}

	options := entities.AutoUpdateOptions{
		Authfile:              authfile,
		DryRun:                query.DryRun,
		Rollback:              query.Rollback,
		InsecureSkipTLSVerify: types.NewOptionalBool(!query.TLSVerify),
	}

	autoUpdateReports, autoUpdateFailures := containerEngine.AutoUpdate(r.Context(), options)
	if autoUpdateReports == nil {
		utils.Error(w, http.StatusInternalServerError, errorhandling.JoinErrors(autoUpdateFailures))
		return
	}

	reports := handlers.LibpodAutoUpdateReports{Reports: autoUpdateReports, Errors: errorhandling.ErrorsToStrings(autoUpdateFailures)}

	utils.WriteResponse(w, http.StatusOK, reports)
}
