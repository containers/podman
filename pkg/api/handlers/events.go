package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/containers/libpod/pkg/api/handlers/utils"
	"github.com/pkg/errors"
)

func GetEvents(w http.ResponseWriter, r *http.Request) {
	query := struct {
		Since   time.Time           `schema:"since"`
		Until   time.Time           `schema:"until"`
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

	libpodEvents, err := getRuntime(r).GetEvents(libpodFilters)
	if err != nil {
		utils.BadRequest(w, "filters", strings.Join(r.URL.Query()["filters"], ", "), err)
		return
	}

	var apiEvents = make([]*Event, len(libpodEvents))
	for _, v := range libpodEvents {
		apiEvents = append(apiEvents, EventToApiEvent(v))
	}
	utils.WriteJSON(w, http.StatusOK, apiEvents)
}
