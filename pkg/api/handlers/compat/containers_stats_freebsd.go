//go:build !remote

package compat

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/containers/podman/v5/libpod"
	"github.com/containers/podman/v5/libpod/define"
	"github.com/containers/podman/v5/pkg/api/handlers/utils"
	api "github.com/containers/podman/v5/pkg/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/sirupsen/logrus"
)

const DefaultStatsPeriod = 5 * time.Second

func StatsContainer(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	decoder := utils.GetDecoder(r)

	query := struct {
		Stream  bool `schema:"stream"`
		OneShot bool `schema:"one-shot"` // added schema for one shot
	}{
		Stream: true,
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}
	if query.Stream && query.OneShot { // mismatch. one-shot can only be passed with stream=false
		utils.Error(w, http.StatusBadRequest, define.ErrInvalidArg)
		return
	}

	name := utils.GetName(r)
	ctnr, err := runtime.LookupContainer(name)
	if err != nil {
		utils.ContainerNotFound(w, name, err)
		return
	}

	stats, err := ctnr.GetContainerStats(nil)
	if err != nil {
		utils.InternalServerError(w, fmt.Errorf("failed to obtain Container %s stats: %w", name, err))
		return
	}

	coder := json.NewEncoder(w)
	// Write header and content type.
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}

	// Set up JSON encoder for streaming.
	coder.SetEscapeHTML(true)
	var preRead time.Time
	var preCPUStats CPUStats
	if query.Stream {
		preRead = time.Now()
		preCPUStats = CPUStats{
			CPUUsage: container.CPUUsage{
				TotalUsage: stats.CPUNano,
			},
			CPU:            stats.CPU,
			OnlineCPUs:     0,
			ThrottlingData: container.ThrottlingData{},
		}
	}

streamLabel: // A label to flatten the scope
	select {
	case <-r.Context().Done():
		logrus.Debugf("Client connection (container stats) cancelled")

	default:
		stats, err = ctnr.GetContainerStats(stats)
		if err != nil {
			logrus.Errorf("Unable to get container stats: %v", err)
			return
		}
		s := StatsJSON{
			Stats: Stats{
				Read:    time.Now(),
				PreRead: preRead,
				CPUStats: CPUStats{
					CPUUsage: container.CPUUsage{
						TotalUsage: stats.CPUNano,
					},
					CPU:            stats.CPU,
					OnlineCPUs:     0,
					ThrottlingData: container.ThrottlingData{},
				},
				PreCPUStats: preCPUStats,
				MemoryStats: container.MemoryStats{},
			},
			Name: stats.Name,
			ID:   stats.ContainerID,
		}

		var jsonOut interface{}
		if utils.IsLibpodRequest(r) {
			jsonOut = s
		} else {
			jsonOut = DockerStatsJSON(s)
		}

		if err := coder.Encode(jsonOut); err != nil {
			logrus.Errorf("Unable to encode stats: %v", err)
			return
		}
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}

		if !query.Stream || query.OneShot {
			return
		}

		preRead = s.Read
		bits, err := json.Marshal(s.CPUStats)
		if err != nil {
			logrus.Errorf("Unable to marshal cpu stats: %q", err)
		}
		if err := json.Unmarshal(bits, &preCPUStats); err != nil {
			logrus.Errorf("Unable to unmarshal previous stats: %q", err)
		}

		time.Sleep(DefaultStatsPeriod)
		goto streamLabel
	}
}
