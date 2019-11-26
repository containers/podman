package serviceapi

import (
	"encoding/json"
	"github.com/sirupsen/logrus"
	"net/http"
	"time"

	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/cgroups"
	docker "github.com/docker/docker/api/types"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
)

const DefaultStatsPeriod = 5 * time.Second

func (s *APIServer) statsContainer(w http.ResponseWriter, r *http.Request) {
	query := struct {
		Stream bool `schema:"stream"`
	}{
		Stream: true,
	}
	if err := s.Decode(&query, r.URL.Query()); err != nil {
		Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
		return
	}

	name := mux.Vars(r)["name"]
	ctnr, err := s.Runtime.LookupContainer(name)
	if err != nil {
		containerNotFound(w, name, err)
		return
	}

	state, err := ctnr.State()
	if err != nil {
		internalServerError(w, err)
		return
	}
	if state != define.ContainerStateRunning && !query.Stream {
		WriteJSON(w, &Stats{docker.StatsJSON{
			Name: ctnr.Name(),
			ID:   ctnr.ID(),
		}})
		return
	}

	var preRead time.Time
	var preCPUStats docker.CPUStats

	stats, err := ctnr.GetContainerStats(&libpod.ContainerStats{})
	if err != nil {
		internalServerError(w, errors.Wrapf(err, "Failed to obtain container %s stats", name))
		return
	}

	if query.Stream {
		preRead = time.Now()
		preCPUStats = docker.CPUStats{
			CPUUsage: docker.CPUUsage{
				TotalUsage:        stats.CPUNano,
				PercpuUsage:       []uint64{uint64(stats.CPU)},
				UsageInKernelmode: 0,
				UsageInUsermode:   0,
			},
			SystemUsage:    0,
			OnlineCPUs:     0,
			ThrottlingData: docker.ThrottlingData{},
		}
		time.Sleep(DefaultStatsPeriod)
	}

	cgroupPath, _ := ctnr.CGroupPath()
	cgroup, _ := cgroups.Load(cgroupPath)

	w.WriteHeader(http.StatusOK)
	for ok := true; ok; ok = query.Stream {
		state, _ := ctnr.State()
		if state != define.ContainerStateRunning {
			time.Sleep(10 * time.Second)
			continue
		}

		stats, _ := ctnr.GetContainerStats(stats)
		cgroupStat, _ := cgroup.Stat()
		inspect, _ := ctnr.Inspect(false)

		net := make(map[string]docker.NetworkStats)
		net[inspect.NetworkSettings.EndpointID] = docker.NetworkStats{
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

		s := Stats{docker.StatsJSON{
			Stats: docker.Stats{
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
				NumProcs: 0,
				StorageStats: docker.StorageStats{
					ReadCountNormalized:  0,
					ReadSizeBytes:        0,
					WriteCountNormalized: 0,
					WriteSizeBytes:       0,
				},
				CPUStats: docker.CPUStats{
					CPUUsage: docker.CPUUsage{
						TotalUsage:        cgroupStat.CPU.Usage.Total,
						PercpuUsage:       []uint64{uint64(stats.CPU)},
						UsageInKernelmode: cgroupStat.CPU.Usage.Kernel,
						UsageInUsermode:   cgroupStat.CPU.Usage.Total - cgroupStat.CPU.Usage.Kernel,
					},
					SystemUsage: 0,
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
		}}

		WriteJSON(w, s)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}

		preRead = s.Read
		bits, err := json.Marshal(s.CPUStats)
		if err != nil {
			logrus.Errorf("unable to marshal cpu stats: %q", err)
		}
		if err := json.Unmarshal(bits, &preCPUStats); err != nil {
			logrus.Errorf("unable to unmarshal previous stats: %q", err)
		}
		time.Sleep(DefaultStatsPeriod)
	}
}

func toBlkioStatEntry(entries []cgroups.BlkIOEntry) []docker.BlkioStatEntry {
	results := make([]docker.BlkioStatEntry, 0, len(entries))
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
