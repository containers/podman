package compat

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/api/handlers/utils"
	"github.com/containers/libpod/pkg/cgroups"
	docker "github.com/docker/docker/api/types"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const DefaultStatsPeriod = 5 * time.Second

func StatsContainer(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	decoder := r.Context().Value("decoder").(*schema.Decoder)

	query := struct {
		Stream bool `schema:"stream"`
	}{
		Stream: true,
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
		return
	}

	name := utils.GetName(r)
	ctnr, err := runtime.LookupContainer(name)
	if err != nil {
		utils.ContainerNotFound(w, name, err)
		return
	}

	// If the container isn't running, then let's not bother and return
	// immediately.
	state, err := ctnr.State()
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	if state != define.ContainerStateRunning {
		utils.Error(w, "Container not running and streaming requested", http.StatusConflict, define.ErrCtrStateInvalid)
		return
	}

	stats, err := ctnr.GetContainerStats(&define.ContainerStats{})
	if err != nil {
		utils.InternalServerError(w, errors.Wrapf(err, "Failed to obtain Container %s stats", name))
		return
	}

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

	for ok := true; ok; ok = query.Stream {
		// Container stats
		stats, err := ctnr.GetContainerStats(stats)
		if err != nil {
			utils.InternalServerError(w, err)
			return
		}
		inspect, err := ctnr.Inspect(false)
		if err != nil {
			utils.InternalServerError(w, err)
			return
		}
		// Cgroup stats
		cgroupPath, err := ctnr.CGroupPath()
		if err != nil {
			utils.InternalServerError(w, err)
			return
		}
		cgroup, err := cgroups.Load(cgroupPath)
		if err != nil {
			utils.InternalServerError(w, err)
			return
		}
		cgroupStat, err := cgroup.Stat()
		if err != nil {
			utils.InternalServerError(w, err)
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

		systemUsage, _ := cgroups.GetSystemCPUUsage()
		s := StatsJSON{
			Stats: Stats{
				Read:    time.Now(),
				PreRead: preRead,
				PidsStats: docker.PidsStats{
					Current: cgroupStat.Pids.Current,
					Limit:   0,
				},
				BlkioStats: docker.BlkioStats{
					IoServiceBytesRecursive: toBlkioStatEntry(cgroupStat.Blkio.IoServiceBytesRecursive),
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
						TotalUsage:        cgroupStat.CPU.Usage.Total,
						PercpuUsage:       cgroupStat.CPU.Usage.PerCPU,
						UsageInKernelmode: cgroupStat.CPU.Usage.Kernel,
						UsageInUsermode:   cgroupStat.CPU.Usage.Total - cgroupStat.CPU.Usage.Kernel,
					},
					CPU:         stats.CPU,
					SystemUsage: systemUsage,
					OnlineCPUs:  uint32(len(cgroupStat.CPU.Usage.PerCPU)),
					ThrottlingData: docker.ThrottlingData{
						Periods:          0,
						ThrottledPeriods: 0,
						ThrottledTime:    0,
					},
				},
				PreCPUStats: preCPUStats,
				MemoryStats: docker.MemoryStats{
					Usage:             cgroupStat.Memory.Usage.Usage,
					MaxUsage:          cgroupStat.Memory.Usage.Limit,
					Stats:             nil,
					Failcnt:           0,
					Limit:             cgroupStat.Memory.Usage.Limit,
					Commit:            0,
					CommitPeak:        0,
					PrivateWorkingSet: 0,
				},
			},
			Name:     stats.Name,
			ID:       stats.ContainerID,
			Networks: net,
		}

		utils.WriteJSON(w, http.StatusOK, s)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}

		preRead = s.Read
		bits, err := json.Marshal(s.CPUStats)
		if err != nil {
			logrus.Errorf("Unable to marshal cpu stats: %q", err)
		}
		if err := json.Unmarshal(bits, &preCPUStats); err != nil {
			logrus.Errorf("Unable to unmarshal previous stats: %q", err)
		}

		// Only sleep when we're streaming.
		if query.Stream {
			time.Sleep(DefaultStatsPeriod)
		}
	}
}

func toBlkioStatEntry(entries []cgroups.BlkIOEntry) []docker.BlkioStatEntry {
	results := make([]docker.BlkioStatEntry, len(entries))
	for i, e := range entries {
		bits, err := json.Marshal(e)
		if err != nil {
			logrus.Errorf("unable to marshal blkio stats: %q", err)
		}
		if err := json.Unmarshal(bits, &results[i]); err != nil {
			logrus.Errorf("unable to unmarshal blkio stats: %q", err)
		}
	}
	return results
}
