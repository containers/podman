package compat

import (
	"fmt"
	"net/http"

	"github.com/containers/podman/v4/libpod"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/api/handlers/utils"
	api "github.com/containers/podman/v4/pkg/api/types"
	"github.com/gorilla/schema"
)

func Changes(w http.ResponseWriter, r *http.Request) {
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)

	query := struct {
		Parent   string `schema:"parent"`
		DiffType string `schema:"diffType"`
	}{}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}
	var diffType define.DiffType
	switch query.DiffType {
	case "", "all":
		diffType = define.DiffAll
	case "container":
		diffType = define.DiffContainer
	case "image":
		diffType = define.DiffImage
	default:
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("invalid diffType value %q", query.DiffType))
		return
	}

	id := utils.GetName(r)
	changes, err := runtime.GetDiff(query.Parent, id, diffType)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	utils.WriteJSON(w, 200, changes)
}
