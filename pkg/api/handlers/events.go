package handlers

import (
	"encoding/json"
	"fmt"
	"github.com/containers/libpod/pkg/api/handlers/utils"
	"net/http"

	"github.com/pkg/errors"
)

func GetEvents(w http.ResponseWriter, r *http.Request) {
	query := struct {
		Since   string `json:"since"`
		Until   string `json:"until"`
		Filters string `json:"filters"`
	}{}
	if err := decodeQuery(r, &query); err != nil {
		utils.Error(w, "Failed to parse parameters", http.StatusBadRequest, errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
	}

	var filters = map[string][]string{}
	if found := hasVar(r, "filters"); found {
		if err := json.Unmarshal([]byte(query.Filters), &filters); err != nil {
			utils.BadRequest(w, "filters", query.Filters, err)
			return
		}
	}

	var libpodFilters = make([]string, len(filters))
	for k, v := range filters {
		libpodFilters = append(libpodFilters, fmt.Sprintf("%s=%s", k, v[0]))
	}

	libpodEvents, err := getRuntime(r).GetEvents(libpodFilters)
	if err != nil {
		utils.BadRequest(w, "filters", query.Filters, err)
		return
	}

	var apiEvents = make([]*Event, len(libpodEvents))
	for _, v := range libpodEvents {
		apiEvents = append(apiEvents, EventToApiEvent(v))
	}
	utils.WriteJSON(w, http.StatusOK, apiEvents)
}
