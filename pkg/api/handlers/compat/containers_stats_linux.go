//go:build !remote

package compat

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/containers/common/pkg/cgroups"
	"github.com/containers/podman/v5/libpod"
	"github.com/containers/podman/v5/libpod/define"
	"github.com/containers/podman/v5/pkg/api/handlers/utils"
	api "github.com/containers/podman/v5/pkg/api/types"
	"github.com/containers/storage/pkg/system"
	"github.com/docker/docker/api/types/container"
	runccgroups "github.com/opencontainers/runc/libcontainer/cgroups"
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
		systemUsage, _ := cgroups.SystemCPUUsage()
		preCPUStats = CPUStats{
			CPUUsage: container.CPUUsage{
				TotalUsage:        stats.CPUNano,
				PercpuUsage:       stats.PerCPU,
				UsageInKernelmode: stats.CPUSystemNano,
				UsageInUsermode:   stats.CPUNano - stats.CPUSystemNano,
			},
			CPU:            stats.CPU,
			SystemUsage:    systemUsage,
			OnlineCPUs:     0,
			ThrottlingData: container.ThrottlingData{},
		}
	}
	onlineCPUs, err := libpod.GetOnlineCPUs(ctnr)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}

streamLabel: // A label to flatten the scope
	select {
	case <-r.Context().Done():
		logrus.Debugf("Client connection (container stats) cancelled")

	default:
		// Container stats
		stats, err = ctnr.GetContainerStats(stats)
		if err != nil {
			logrus.Errorf("Unable to get container stats: %v", err)
			return
		}
		inspect, err := ctnr.Inspect(false)
		if err != nil {
			logrus.Errorf("Unable to inspect container: %v", err)
			return
		}
		// Cgroup stats
		cgroupPath, err := ctnr.CgroupPath()
		if err != nil {
			logrus.Errorf("Unable to get cgroup path of container: %v", err)
			return
		}
		cgroup, err := cgroups.Load(cgroupPath)
		if err != nil {
			logrus.Errorf("Unable to load cgroup: %v", err)
			return
		}
		cgroupStat, err := cgroup.Stat()
		if err != nil {
			logrus.Errorf("Unable to get cgroup stats: %v", err)
			return
		}

		net := make(map[string]container.NetworkStats)
		for netName, netStats := range stats.Network {
			net[netName] = container.NetworkStats{
				RxBytes:    netStats.RxBytes,
				RxPackets:  netStats.RxPackets,
				RxErrors:   netStats.RxErrors,
				RxDropped:  netStats.RxDropped,
				TxBytes:    netStats.TxBytes,
				TxPackets:  netStats.TxPackets,
				TxErrors:   netStats.TxErrors,
				TxDropped:  netStats.TxDropped,
				EndpointID: inspect.NetworkSettings.EndpointID,
				InstanceID: "",
			}
		}

		resources := ctnr.LinuxResources()
		memoryLimit := cgroupStat.MemoryStats.Usage.Limit
		if resources != nil && resources.Memory != nil && *resources.Memory.Limit > 0 {
			memoryLimit = uint64(*resources.Memory.Limit)
		}

		memInfo, err := system.ReadMemInfo()
		if err != nil {
			logrus.Errorf("Unable to get cgroup stats: %v", err)
			return
		}
		// cap the memory limit to the available memory.
		if memInfo.MemTotal > 0 && memoryLimit > uint64(memInfo.MemTotal) {
			memoryLimit = uint64(memInfo.MemTotal)
		}

		systemUsage, _ := cgroups.SystemCPUUsage()
		s := StatsJSON{
			Stats: Stats{
				Read:    time.Now(),
				PreRead: preRead,
				PidsStats: container.PidsStats{
					Current: cgroupStat.PidsStats.Current,
					Limit:   0,
				},
				BlkioStats: container.BlkioStats{
					IoServiceBytesRecursive: toBlkioStatEntry(cgroupStat.BlkioStats.IoServiceBytesRecursive),
					IoServicedRecursive:     nil,
					IoQueuedRecursive:       nil,
					IoServiceTimeRecursive:  nil,
					IoWaitTimeRecursive:     nil,
					IoMergedRecursive:       nil,
					IoTimeRecursive:         nil,
					SectorsRecursive:        nil,
				},
				CPUStats: CPUStats{
					CPUUsage: container.CPUUsage{
						TotalUsage:        cgroupStat.CpuStats.CpuUsage.TotalUsage,
						PercpuUsage:       cgroupStat.CpuStats.CpuUsage.PercpuUsage,
						UsageInKernelmode: cgroupStat.CpuStats.CpuUsage.UsageInKernelmode,
						UsageInUsermode:   cgroupStat.CpuStats.CpuUsage.TotalUsage - cgroupStat.CpuStats.CpuUsage.UsageInKernelmode,
					},
					CPU:         stats.CPU,
					SystemUsage: systemUsage,
					OnlineCPUs:  uint32(onlineCPUs),
					ThrottlingData: container.ThrottlingData{
						Periods:          0,
						ThrottledPeriods: 0,
						ThrottledTime:    0,
					},
				},
				PreCPUStats: preCPUStats,
				MemoryStats: container.MemoryStats{
					Usage:             cgroupStat.MemoryStats.Usage.Usage,
					MaxUsage:          cgroupStat.MemoryStats.Usage.MaxUsage,
					Stats:             nil,
					Failcnt:           0,
					Limit:             memoryLimit,
					Commit:            0,
					CommitPeak:        0,
					PrivateWorkingSet: 0,
				},
			},
			Name:     stats.Name,
			ID:       stats.ContainerID,
			Networks: net,
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

func toBlkioStatEntry(entries []runccgroups.BlkioStatEntry) []container.BlkioStatEntry {
	results := make([]container.BlkioStatEntry, len(entries))
	for i, e := range entries {
		bits, err := json.Marshal(e)
		if err != nil {
			logrus.Errorf("Unable to marshal blkio stats: %q", err)
		}
		if err := json.Unmarshal(bits, &results[i]); err != nil {
			logrus.Errorf("Unable to unmarshal blkio stats: %q", err)
		}
	}
	return results
}
