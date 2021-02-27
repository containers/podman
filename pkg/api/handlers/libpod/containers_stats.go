package libpod

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/containers/podman/v3/libpod"
	"github.com/containers/podman/v3/pkg/api/handlers/utils"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/containers/podman/v3/pkg/domain/infra/abi"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const DefaultStatsPeriod = 5 * time.Second

func StatsContainer(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	decoder := r.Context().Value("decoder").(*schema.Decoder)

	query := struct {
		Containers []string `schema:"containers"`
		Stream     bool     `schema:"stream"`
	}{
		Stream: true,
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}

	// Reduce code duplication and use the local/abi implementation of
	// container stats.
	containerEngine := abi.ContainerEngine{Libpod: runtime}

	statsOptions := entities.ContainerStatsOptions{
		Stream: query.Stream,
	}

	// Stats will stop if the connection is closed.
	statsChan, err := containerEngine.ContainerStats(r.Context(), query.Containers, statsOptions)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}

	// Write header and content type.
	w.WriteHeader(http.StatusOK)
	w.Header().Add("Content-Type", "application/json")
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}

	// Setup JSON encoder for streaming.
	coder := json.NewEncoder(w)
	coder.SetEscapeHTML(true)

	for stats := range statsChan {
		if err := coder.Encode(stats); err != nil {
			// Note: even when streaming, the stats goroutine will
			// be notified (and stop) as the connection will be
			// closed.
			logrus.Errorf("Unable to encode stats: %v", err)
			return
		}
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
	}
}
