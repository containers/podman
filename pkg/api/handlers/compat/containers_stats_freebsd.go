//go:build !remote

package compat

import (
	"time"

	"github.com/containers/podman/v5/libpod"
	"github.com/containers/podman/v5/libpod/define"
	"github.com/docker/docker/api/types/container"
)

func getPreCPUStats(stats *define.ContainerStats) CPUStats {
	return CPUStats{
		CPUUsage: container.CPUUsage{
			TotalUsage: stats.CPUNano,
		},
		CPU:            stats.CPU,
		OnlineCPUs:     0,
		ThrottlingData: container.ThrottlingData{},
	}
}

func statsContainerJSON(ctnr *libpod.Container, stats *define.ContainerStats, preCPUStats CPUStats, onlineCPUs int) (StatsJSON, error) {
	return StatsJSON{
		Stats: Stats{
			Read: time.Now(),
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
	}, nil
}
