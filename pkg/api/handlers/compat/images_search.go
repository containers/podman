package compat

import (
	"fmt"
	"net/http"

	"github.com/containers/image/v5/types"
	"github.com/containers/podman/v3/libpod"
	"github.com/containers/podman/v3/pkg/api/handlers/utils"
	"github.com/containers/podman/v3/pkg/auth"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/containers/podman/v3/pkg/domain/infra/abi"
	"github.com/containers/storage"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
)

func SearchImages(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	query := struct {
		Term      string              `json:"term"`
		Limit     int                 `json:"limit"`
		NoTrunc   bool                `json:"noTrunc"`
		Filters   map[string][]string `json:"filters"`
		TLSVerify bool                `json:"tlsVerify"`
		ListTags  bool                `json:"listTags"`
	}{
		// This is where you can override the golang default value for one of fields
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}

	_, authfile, key, err := auth.GetCredentials(r)
	if err != nil {
		utils.Error(w, "failed to retrieve repository credentials", http.StatusBadRequest, errors.Wrapf(err, "failed to parse %q header for %s", key, r.URL.String()))
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
		NoTrunc:  query.NoTrunc,
		ListTags: query.ListTags,
		Filters:  filters,
	}
	if _, found := r.URL.Query()["tlsVerify"]; found {
		options.SkipTLSVerify = types.NewOptionalBool(!query.TLSVerify)
	}
	ir := abi.ImageEngine{Libpod: runtime}
	reports, err := ir.Search(r.Context(), query.Term, options)
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, err)
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
