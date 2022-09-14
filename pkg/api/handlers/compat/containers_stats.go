package compat

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/containers/common/pkg/cgroups"
	"github.com/containers/podman/v4/libpod"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/api/handlers/utils"
	api "github.com/containers/podman/v4/pkg/api/types"
	"github.com/containers/storage/pkg/system"
	docker "github.com/docker/docker/api/types"
	"github.com/gorilla/schema"
	runccgroups "github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/sirupsen/logrus"
)

const DefaultStatsPeriod = 5 * time.Second

func StatsContainer(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)

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
		systemUsage, _ := cgroups.GetSystemCPUUsage()
		preCPUStats = CPUStats{
			CPUUsage: docker.CPUUsage{
				TotalUsage:        stats.CPUNano,
				PercpuUsage:       stats.PerCPU,
				UsageInKernelmode: stats.CPUSystemNano,
				UsageInUsermode:   stats.CPUNano - stats.CPUSystemNano,
			},
			CPU:            stats.CPU,
			SystemUsage:    systemUsage,
			OnlineCPUs:     0,
			ThrottlingData: docker.ThrottlingData{},
		}
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

		// FIXME: network inspection does not yet work entirely
		net := make(map[string]docker.NetworkStats)
		networkName := inspect.NetworkSettings.EndpointID
		if networkName == "" {
			networkName = "network"
		}
		net[networkName] = docker.NetworkStats{
			RxBytes:    stats.NetInput,
			RxPackets:  0,
			RxErrors:   0,
			RxDropped:  0,
			TxBytes:    stats.NetOutput,
			TxPackets:  0,
			TxErrors:   0,
			TxDropped:  0,
			EndpointID: inspect.NetworkSettings.EndpointID,
			InstanceID: "",
		}

		cfg := ctnr.Config()
		memoryLimit := cgroupStat.MemoryStats.Usage.Limit
		if cfg.Spec.Linux != nil && cfg.Spec.Linux.Resources != nil && cfg.Spec.Linux.Resources.Memory != nil && *cfg.Spec.Linux.Resources.Memory.Limit > 0 {
			memoryLimit = uint64(*cfg.Spec.Linux.Resources.Memory.Limit)
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

		systemUsage, _ := cgroups.GetSystemCPUUsage()
		s := StatsJSON{
			Stats: Stats{
				Read:    time.Now(),
				PreRead: preRead,
				PidsStats: docker.PidsStats{
					Current: cgroupStat.PidsStats.Current,
					Limit:   0,
				},
				BlkioStats: docker.BlkioStats{
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
					CPUUsage: docker.CPUUsage{
						TotalUsage:        cgroupStat.CpuStats.CpuUsage.TotalUsage,
						PercpuUsage:       cgroupStat.CpuStats.CpuUsage.PercpuUsage,
						UsageInKernelmode: cgroupStat.CpuStats.CpuUsage.UsageInKernelmode,
						UsageInUsermode:   cgroupStat.CpuStats.CpuUsage.TotalUsage - cgroupStat.CpuStats.CpuUsage.UsageInKernelmode,
					},
					CPU:         stats.CPU,
					SystemUsage: systemUsage,
					OnlineCPUs:  uint32(len(cgroupStat.CpuStats.CpuUsage.PercpuUsage)),
					ThrottlingData: docker.ThrottlingData{
						Periods:          0,
						ThrottledPeriods: 0,
						ThrottledTime:    0,
					},
				},
				PreCPUStats: preCPUStats,
				MemoryStats: docker.MemoryStats{
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

		if err := coder.Encode(s); err != nil {
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

func toBlkioStatEntry(entries []runccgroups.BlkioStatEntry) []docker.BlkioStatEntry {
	results := make([]docker.BlkioStatEntry, len(entries))
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
