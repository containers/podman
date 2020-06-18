package compat

import (
	"context"
	"fmt"
	"net/http"

	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/events"
	"github.com/containers/libpod/pkg/api/handlers/utils"
	"github.com/containers/libpod/pkg/domain/entities"
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
	}

	var libpodFilters = []string{}
	if _, found := r.URL.Query()["filters"]; found {
		for k, v := range query.Filters {
			libpodFilters = append(libpodFilters, fmt.Sprintf("%s=%s", k, v[0]))
		}
	}

	if len(query.Since) > 0 || len(query.Until) > 0 {
		fromStart = true
	}

	eventCtx, eventCancel := context.WithCancel(r.Context())
	eventChannel := make(chan *events.Event)
	defer close(eventChannel)

	// If client disappears we need to stop listening for events
	go func(done <-chan struct{}) {
		<-done
		if _, ok := <-eventChannel; ok {
			eventCancel()
		}
	}(r.Context().Done())

	// proxy events from runtime to http response
	go func() {
		readOpts := events.ReadOptions{FromStart: fromStart, Stream: query.Stream, Filters: libpodFilters, EventChannel: eventChannel, Since: query.Since, Until: query.Until}
		if err := runtime.Events(eventCtx, readOpts); err != nil {
			logrus.Warnf("Failed to process event %q", err.Error())
		}
		eventCancel()
	}()

	// Headers need to be written out before turning Writer() over to json encoder
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}

	json := jsoniter.ConfigCompatibleWithStandardLibrary
	coder := json.NewEncoder(w)
	coder.SetEscapeHTML(true)

	for {
		select {
		case event := <-eventChannel:
			// We got an event keep going
			e := entities.ConvertToEntitiesEvent(*event)
			if err := coder.Encode(e); err != nil {
				logrus.Errorf("unable to encode event json %q", err.Error())
				continue
			}
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		case <-eventCtx.Done():
			return
		}
	}
}
