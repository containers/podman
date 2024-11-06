//go:build !remote

package compat

import (
	"fmt"
	"net/http"

	"github.com/containers/podman/v5/libpod"
	"github.com/containers/podman/v5/libpod/events"
	"github.com/containers/podman/v5/pkg/api/handlers/utils"
	api "github.com/containers/podman/v5/pkg/api/types"
	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/containers/podman/v5/pkg/util"
	jsoniter "github.com/json-iterator/go"
	"github.com/sirupsen/logrus"
)

// GetEvents endpoint serves both the docker-compatible one and the new libpod one
func GetEvents(w http.ResponseWriter, r *http.Request) {
	var (
		fromStart bool
		decoder   = utils.GetDecoder(r)
		runtime   = r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
		json      = jsoniter.ConfigCompatibleWithStandardLibrary // FIXME: this should happen on the package level
	)

	// NOTE: the "filters" parameter is extracted separately for backwards
	// compat via `filterFromRequest()`.
	query := struct {
		Since  string `schema:"since"`
		Until  string `schema:"until"`
		Stream bool   `schema:"stream"`
	}{
		Stream: true,
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	if len(query.Since) > 0 || len(query.Until) > 0 {
		fromStart = true
	}

	libpodFilters, err := util.FiltersFromRequest(r)
	if err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse filters for %s: %w", r.URL.String(), err))
		return
	}
	eventChannel := make(chan events.ReadResult)

	readOpts := events.ReadOptions{
		FromStart:    fromStart,
		Stream:       query.Stream,
		Filters:      libpodFilters,
		EventChannel: eventChannel,
		Since:        query.Since,
		Until:        query.Until,
	}
	err = runtime.Events(r.Context(), readOpts)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}

	flush := func() {}
	if flusher, ok := w.(http.Flusher); ok {
		flush = flusher.Flush
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	flush()

	coder := json.NewEncoder(w)
	coder.SetEscapeHTML(true)

	for {
		select {
		case <-r.Context().Done():
			return
		case evt, ok := <-eventChannel:
			if !ok {
				return
			}
			if evt.Error != nil {
				logrus.Errorf("Unable to read event: %q", err)
				continue
			}
			if evt.Event == nil {
				continue
			}

			e := entities.ConvertToEntitiesEvent(*evt.Event)
			// Some events differ between Libpod and Docker endpoints.
			// Handle these differences for Docker-compat.
			if !utils.IsLibpodRequest(r) && e.Type == "image" && e.Status == "remove" {
				e.Status = "delete"
				e.Action = "delete"
			}
			if !utils.IsLibpodRequest(r) && e.Status == "died" {
				e.Status = "die"
				e.Action = "die"
				e.Actor.Attributes["exitCode"] = e.Actor.Attributes["containerExitCode"]
			}

			if err := coder.Encode(e); err != nil {
				logrus.Errorf("Unable to write json: %q", err)
			}
			flush()
		}
	}
}
