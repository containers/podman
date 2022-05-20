package compat

import (
	"net/http"

	"github.com/containers/podman/v4/libpod"
	"github.com/containers/podman/v4/libpod/events"
	"github.com/containers/podman/v4/pkg/api/handlers/utils"
	api "github.com/containers/podman/v4/pkg/api/types"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/util"
	"github.com/gorilla/schema"
	jsoniter "github.com/json-iterator/go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// GetEvents endpoint serves both the docker-compatible one and the new libpod one
func GetEvents(w http.ResponseWriter, r *http.Request) {
	var (
		fromStart bool
		decoder   = r.Context().Value(api.DecoderKey).(*schema.Decoder)
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
		utils.Error(w, http.StatusBadRequest, errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}

	if len(query.Since) > 0 || len(query.Until) > 0 {
		fromStart = true
	}

	libpodFilters, err := util.FiltersFromRequest(r)
	if err != nil {
		utils.Error(w, http.StatusBadRequest, errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}
	eventChannel := make(chan *events.Event)
	errorChannel := make(chan error)

	// Start reading events.
	go func() {
		readOpts := events.ReadOptions{
			FromStart:    fromStart,
			Stream:       query.Stream,
			Filters:      libpodFilters,
			EventChannel: eventChannel,
			Since:        query.Since,
			Until:        query.Until,
		}
		errorChannel <- runtime.Events(r.Context(), readOpts)
	}()

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
		case err := <-errorChannel:
			if err != nil {
				// FIXME StatusOK already sent above cannot send 500 here
				utils.InternalServerError(w, err)
			}
			return
		case evt := <-eventChannel:
			if evt == nil {
				continue
			}

			e := entities.ConvertToEntitiesEvent(*evt)
			if !utils.IsLibpodRequest(r) && e.Status == "died" {
				e.Status = "die"
				e.Action = "die"
				e.Actor.Attributes["exitCode"] = e.Actor.Attributes["containerExitCode"]
			}

			if err := coder.Encode(e); err != nil {
				logrus.Errorf("Unable to write json: %q", err)
			}
			flush()
		case <-r.Context().Done():
			return
		}
	}
}
