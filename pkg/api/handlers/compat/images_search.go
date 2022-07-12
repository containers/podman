package compat

import (
	"fmt"
	"net/http"

	"github.com/containers/image/v5/types"
	"github.com/containers/podman/v4/libpod"
	"github.com/containers/podman/v4/pkg/api/handlers/utils"
	api "github.com/containers/podman/v4/pkg/api/types"
	"github.com/containers/podman/v4/pkg/auth"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/domain/infra/abi"
	"github.com/containers/storage"
	"github.com/gorilla/schema"
)

func SearchImages(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)
	query := struct {
		Term      string              `json:"term"`
		Limit     int                 `json:"limit"`
		Filters   map[string][]string `json:"filters"`
		TLSVerify bool                `json:"tlsVerify"`
		ListTags  bool                `json:"listTags"`
	}{
		// This is where you can override the golang default value for one of fields
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

	filters := []string{}
	for key, val := range query.Filters {
		filters = append(filters, fmt.Sprintf("%s=%s", key, val[0]))
	}

	options := entities.ImageSearchOptions{
		Authfile: authfile,
		Limit:    query.Limit,
		ListTags: query.ListTags,
		Filters:  filters,
	}
	if _, found := r.URL.Query()["tlsVerify"]; found {
		options.SkipTLSVerify = types.NewOptionalBool(!query.TLSVerify)
	}
	ir := abi.ImageEngine{Libpod: runtime}
	reports, err := ir.Search(r.Context(), query.Term, options)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, err)
		return
	}
	if !utils.IsLibpodRequest(r) {
		if len(reports) == 0 {
			utils.ImageNotFound(w, query.Term, storage.ErrImageUnknown)
			return
		}
	}

	utils.WriteResponse(w, http.StatusOK, reports)
}
