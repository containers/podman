package compat

import (
	"net/http"
	"strconv"

	"github.com/containers/image/v5/types"
	"github.com/containers/podman/v2/libpod/image"
	"github.com/containers/podman/v2/pkg/api/handlers/utils"
	"github.com/containers/podman/v2/pkg/auth"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
)

func SearchImages(w http.ResponseWriter, r *http.Request) {
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	query := struct {
		Term      string              `json:"term"`
		Limit     int                 `json:"limit"`
		Filters   map[string][]string `json:"filters"`
		TLSVerify bool                `json:"tlsVerify"`
	}{
		// This is where you can override the golang default value for one of fields
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}

	filter := image.SearchFilter{}
	if len(query.Filters) > 0 {
		if len(query.Filters["stars"]) > 0 {
			stars, err := strconv.Atoi(query.Filters["stars"][0])
			if err != nil {
				utils.InternalServerError(w, err)
				return
			}
			filter.Stars = stars
		}
		if len(query.Filters["is-official"]) > 0 {
			isOfficial, err := strconv.ParseBool(query.Filters["is-official"][0])
			if err != nil {
				utils.InternalServerError(w, err)
				return
			}
			filter.IsOfficial = types.NewOptionalBool(isOfficial)
		}
		if len(query.Filters["is-automated"]) > 0 {
			isAutomated, err := strconv.ParseBool(query.Filters["is-automated"][0])
			if err != nil {
				utils.InternalServerError(w, err)
				return
			}
			filter.IsAutomated = types.NewOptionalBool(isAutomated)
		}
	}
	options := image.SearchOptions{
		Filter: filter,
		Limit:  query.Limit,
	}

	if _, found := r.URL.Query()["tlsVerify"]; found {
		options.InsecureSkipTLSVerify = types.NewOptionalBool(!query.TLSVerify)
	}

	_, authfile, key, err := auth.GetCredentials(r)
	if err != nil {
		utils.Error(w, "failed to retrieve repository credentials", http.StatusBadRequest, errors.Wrapf(err, "failed to parse %q header for %s", key, r.URL.String()))
		return
	}
	defer auth.RemoveAuthfile(authfile)
	options.Authfile = authfile

	results, err := image.SearchImages(query.Term, options)
	if err != nil {
		utils.BadRequest(w, "term", query.Term, err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, results)
}
