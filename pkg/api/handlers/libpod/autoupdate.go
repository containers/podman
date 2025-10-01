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
		DryRun    bool               `schema:"dryRun"`
		Rollback  bool               `schema:"rollback"`
		TLSVerify types.OptionalBool `schema:"tlsVerify"`
	}{}

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
		InsecureSkipTLSVerify: types.OptionalBoolUndefined,
	}

	// If TLS verification is explicitly specified (True or False) in the query,
	// set the InsecureSkipTLSVerify option accordingly.
	// If TLSVerify was not set in the query, OptionalBoolUndefined is used and
	// handled later based off the target registry configuration.
	switch query.TLSVerify {
	case types.OptionalBoolTrue:
		options.InsecureSkipTLSVerify = types.NewOptionalBool(false)
	case types.OptionalBoolFalse:
		options.InsecureSkipTLSVerify = types.NewOptionalBool(true)
	case types.OptionalBoolUndefined:
		// If the user doesn't define TLSVerify in the query, do nothing and pass
		// it to the backend code to handle.
	default: // Should never happen
		panic("Unexpected handling occurred for TLSVerify")
	}

	autoUpdateReports, autoUpdateFailures := containerEngine.AutoUpdate(r.Context(), options)
	if autoUpdateReports == nil {
		if err := errorhandling.JoinErrors(autoUpdateFailures); err != nil {
			utils.Error(w, http.StatusInternalServerError, err)
			return
		}
	}

	reports := handlers.LibpodAutoUpdateReports{Reports: autoUpdateReports, Errors: errorhandling.ErrorsToStrings(autoUpdateFailures)}

	utils.WriteResponse(w, http.StatusOK, reports)
}
