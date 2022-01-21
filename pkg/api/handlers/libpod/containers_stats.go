package libpod

import (
	"encoding/json"
	"net/http"

	"github.com/containers/common/pkg/cgroups"
	"github.com/containers/podman/v4/libpod"
	"github.com/containers/podman/v4/pkg/api/handlers/utils"
	api "github.com/containers/podman/v4/pkg/api/types"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/domain/infra/abi"
	"github.com/containers/podman/v4/pkg/rootless"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func StatsContainer(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)

	// Check if service is running rootless (cheap check)
	if rootless.IsRootless() {
		// if so, then verify cgroup v2 available (more expensive check)
		if isV2, _ := cgroups.IsCgroup2UnifiedMode(); !isV2 {
			msg := "Container stats resource only available for cgroup v2"
			utils.Error(w, http.StatusConflict, errors.New(msg))
			return
		}
	}

	query := struct {
		Containers []string `schema:"containers"`
		Stream     bool     `schema:"stream"`
		Interval   int      `schema:"interval"`
	}{
		Stream:   true,
		Interval: 5,
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}

	// Reduce code duplication and use the local/abi implementation of
	// container stats.
	containerEngine := abi.ContainerEngine{Libpod: runtime}

	statsOptions := entities.ContainerStatsOptions{
		Stream:   query.Stream,
		Interval: query.Interval,
	}

	// Stats will stop if the connection is closed.
	statsChan, err := containerEngine.ContainerStats(r.Context(), query.Containers, statsOptions)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}

	// Write header and content type.
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
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
