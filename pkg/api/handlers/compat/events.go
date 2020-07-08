package compat

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/containers/libpod/v2/libpod"
	"github.com/containers/libpod/v2/libpod/events"
	"github.com/containers/libpod/v2/pkg/api/handlers/utils"
	"github.com/containers/libpod/v2/pkg/domain/entities"
	"github.com/gorilla/schema"
	jsoniter "github.com/json-iterator/go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func GetEvents(w http.ResponseWriter, r *http.Request) {
	var (
		fromStart bool
		decoder   = r.Context().Value("decoder").(*schema.Decoder)
		runtime   = r.Context().Value("runtime").(*libpod.Runtime)
		json      = jsoniter.ConfigCompatibleWithStandardLibrary // FIXME: this should happen on the package level
	)

	query := struct {
		Since   string              `schema:"since"`
		Until   string              `schema:"until"`
		Filters map[string][]string `schema:"filters"`
		Stream  bool                `schema:"stream"`
	}{
		Stream: true,
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, "Failed to parse parameters", http.StatusBadRequest, errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
		return
	}

	var libpodFilters = []string{}
	if _, found := r.URL.Query()["filters"]; found {
		for k, v := range query.Filters {
			if len(v) == 0 {
				utils.Error(w, "Failed to parse parameters", http.StatusBadRequest, errors.Errorf("empty value for filter %q", k))
				return
			}
			libpodFilters = append(libpodFilters, fmt.Sprintf("%s=%s", k, v[0]))
		}
	}

	if len(query.Since) > 0 || len(query.Until) > 0 {
		fromStart = true
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

	var coder *jsoniter.Encoder
	var writeHeader sync.Once

	for stream := true; stream; stream = query.Stream {
		select {
		case err := <-errorChannel:
			if err != nil {
				utils.InternalServerError(w, err)
				return
			}
		case evt := <-eventChannel:
			writeHeader.Do(func() {
				// Use a sync.Once so that we write the header
				// only once.
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				if flusher, ok := w.(http.Flusher); ok {
					flusher.Flush()
				}
				coder = json.NewEncoder(w)
				coder.SetEscapeHTML(true)
			})

			if evt == nil {
				continue
			}

			e := entities.ConvertToEntitiesEvent(*evt)
			if err := coder.Encode(e); err != nil {
				logrus.Errorf("unable to write json: %q", err)
			}
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		}

	}
}
