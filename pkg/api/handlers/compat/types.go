package compat

import (
	"time"

	docker "github.com/docker/docker/api/types"
)

// CPUStats aggregates and wraps all CPU related info of container
type CPUStats struct {
	// CPU Usage. Linux and Windows.
	CPUUsage docker.CPUUsage `json:"cpu_usage"`

	// System Usage. Linux only.
	SystemUsage uint64 `json:"system_cpu_usage,omitempty"`

	// Online CPUs. Linux only.
	OnlineCPUs uint32 `json:"online_cpus,omitempty"`

	// Usage of CPU in %. Linux only.
	CPU float64 `json:"cpu"`

	// Throttling Data. Linux only.
	ThrottlingData docker.ThrottlingData `json:"throttling_data,omitempty"`
}

// Stats is Ultimate struct aggregating all types of stats of one container
type Stats struct {
	// Common stats
	Read    time.Time `json:"read"`
	PreRead time.Time `json:"preread"`

	// Linux specific stats, not populated on Windows.
	PidsStats  docker.PidsStats  `json:"pids_stats,omitempty"`
	BlkioStats docker.BlkioStats `json:"blkio_stats,omitempty"`

	// Windows specific stats, not populated on Linux.
	NumProcs     uint32              `json:"num_procs"`
	StorageStats docker.StorageStats `json:"storage_stats,omitempty"`

	// Shared stats
	CPUStats    CPUStats           `json:"cpu_stats,omitempty"`
	PreCPUStats CPUStats           `json:"precpu_stats,omitempty"` // "Pre"="Previous"
	MemoryStats docker.MemoryStats `json:"memory_stats,omitempty"`
}

type StatsJSON struct {
	Stats

	Name string `json:"name,omitempty"`
	ID   string `json:"Id,omitempty"`

	// Networks request version >=1.21
	Networks map[string]docker.NetworkStats `json:"networks,omitempty"`
}
