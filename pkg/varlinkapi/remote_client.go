// +build varlink remoteclient

package varlinkapi

import (
	"github.com/containers/libpod/cmd/podman/varlink"
	"github.com/containers/libpod/libpod"
)

// ContainerStatsToLibpodContainerStats converts the varlink containerstats to a libpod
// container stats
func ContainerStatsToLibpodContainerStats(stats iopodman.ContainerStats) libpod.ContainerStats {
	cstats := libpod.ContainerStats{
		ContainerID: stats.Id,
		Name:        stats.Name,
		CPU:         stats.Cpu,
		CPUNano:     uint64(stats.Cpu_nano),
		SystemNano:  uint64(stats.System_nano),
		MemUsage:    uint64(stats.Mem_usage),
		MemLimit:    uint64(stats.Mem_limit),
		MemPerc:     stats.Mem_perc,
		NetInput:    uint64(stats.Net_input),
		NetOutput:   uint64(stats.Net_output),
		BlockInput:  uint64(stats.Block_input),
		BlockOutput: uint64(stats.Block_output),
		PIDs:        uint64(stats.Pids),
	}
	return cstats
}
