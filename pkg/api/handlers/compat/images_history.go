package compat

import (
	"net/http"

	"github.com/containers/common/libimage"
	"github.com/containers/podman/v3/libpod"
	"github.com/containers/podman/v3/pkg/api/handlers"
	"github.com/containers/podman/v3/pkg/api/handlers/utils"
	"github.com/pkg/errors"
)

func HistoryImage(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	name := utils.GetName(r)

	lookupOptions := &libimage.LookupImageOptions{IgnorePlatform: true}
	newImage, _, err := runtime.LibimageRuntime().LookupImage(name, lookupOptions)
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusNotFound, errors.Wrapf(err, "failed to find image %s", name))
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
