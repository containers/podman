package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/containers/libpod/libpod/events"
	"github.com/containers/libpod/pkg/api/handlers/utils"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func GetEvents(w http.ResponseWriter, r *http.Request) {
	var (
		fromStart   bool
		eventsError error
	)
	query := struct {
		Since   string              `schema:"since"`
		Until   string              `schema:"until"`
		Filters map[string][]string `schema:"filters"`
	}{}
	if err := decodeQuery(r, &query); err != nil {
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
	eventChannel := make(chan *events.Event)
	go func() {
		readOpts := events.ReadOptions{FromStart: fromStart, Stream: true, Filters: libpodFilters, EventChannel: eventChannel, Since: query.Since, Until: query.Until}
		eventsError = getRuntime(r).Events(readOpts)
	}()
	if eventsError != nil {
		utils.InternalServerError(w, eventsError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	for event := range eventChannel {
		e := EventToApiEvent(event)
		//utils.WriteJSON(w, http.StatusOK, e)
		coder := json.NewEncoder(w)
		coder.SetEscapeHTML(true)
		if err := coder.Encode(e); err != nil {
			logrus.Errorf("unable to write json: %q", err)
		}
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
	}
}
