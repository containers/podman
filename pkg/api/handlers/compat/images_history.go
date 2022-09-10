package compat

import (
	"fmt"
	"net/http"

	"github.com/containers/podman/v4/libpod"
	"github.com/containers/podman/v4/pkg/api/handlers"
	"github.com/containers/podman/v4/pkg/api/handlers/utils"
	api "github.com/containers/podman/v4/pkg/api/types"
)

func HistoryImage(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	name := utils.GetName(r)

	possiblyNormalizedName, err := utils.NormalizeToDockerHub(r, name)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("normalizing image: %w", err))
		return
	}

	newImage, _, err := runtime.LibimageRuntime().LookupImage(possiblyNormalizedName, nil)
	if err != nil {
		utils.ImageNotFound(w, possiblyNormalizedName, fmt.Errorf("failed to find image %s: %w", possiblyNormalizedName, err))
		return
	}
	history, err := newImage.History(r.Context())
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	allHistory := make([]handlers.HistoryResponse, 0, len(history))
	for _, h := range history {
		l := handlers.HistoryResponse{
			ID:        h.ID,
			Created:   h.Created.Unix(),
			CreatedBy: h.CreatedBy,
			Tags:      h.Tags,
			Size:      h.Size,
			Comment:   h.Comment,
		}
		allHistory = append(allHistory, l)
	}
	utils.WriteResponse(w, http.StatusOK, allHistory)
}
