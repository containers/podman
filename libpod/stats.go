// +build linux

package libpod

import (
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/cgroups"
	"github.com/pkg/errors"
)

// GetContainerStats gets the running stats for a given container
func (c *Container) GetContainerStats(previousStats *ContainerStats) (*ContainerStats, error) {
	stats := new(ContainerStats)
	stats.ContainerID = c.ID()
	stats.Name = c.Name()

	if c.config.NoCgroups {
		return nil, errors.Wrapf(define.ErrNoCgroups, "cannot run top on container %s as it did not create a cgroup", c.ID())
	}

	if !c.batched {
		c.lock.Lock()
		defer c.lock.Unlock()
		if err := c.syncContainer(); err != nil {
			return stats, err
		}
	}

	if c.state.State != define.ContainerStateRunning {
		return stats, define.ErrCtrStateInvalid
	}

	cgroupPath, err := c.CGroupPath()
	if err != nil {
		return nil, err
	}
	cgroup, err := cgroups.Load(cgroupPath)
	if err != nil {
		return stats, errors.Wrapf(err, "unable to load cgroup at %s", cgroupPath)
	}

	// Ubuntu does not have swap memory in cgroups because swap is often not enabled.
	cgroupStats, err := cgroup.Stat()
	if err != nil {
		return stats, errors.Wrapf(err, "unable to obtain cgroup stats")
	}
	conState := c.state.State
	netStats, err := getContainerNetIO(c)
	if err != nil {
		return nil, err
	}

	previousCPU := previousStats.CPUNano
	previousSystem := previousStats.SystemNano
	stats.CPU = calculateCPUPercent(cgroupStats, previousCPU, previousSystem)
	stats.MemUsage = cgroupStats.Memory.Usage.Usage
	stats.MemLimit = getMemLimit(cgroupStats.Memory.Usage.Limit)
	stats.MemPerc = (float64(stats.MemUsage) / float64(stats.MemLimit)) * 100
	stats.PIDs = 0
	if conState == define.ContainerStateRunning {
		stats.PIDs = cgroupStats.Pids.Current
	}
	stats.BlockInput, stats.BlockOutput = calculateBlockIO(cgroupStats)
	stats.CPUNano = cgroupStats.CPU.Usage.Total
	stats.SystemNano = cgroupStats.CPU.Usage.Kernel
	// Handle case where the container is not in a network namespace
	if netStats != nil {
		stats.NetInput = netStats.TxBytes
		stats.NetOutput = netStats.RxBytes
	} else {
		stats.NetInput = 0
		stats.NetOutput = 0
	}

	return stats, nil
}

// getMemory limit returns the memory limit for a given cgroup
// If the configured memory limit is larger than the total memory on the sys, the
// physical system memory size is returned
func getMemLimit(cgroupLimit uint64) uint64 {
	si := &syscall.Sysinfo_t{}
	err := syscall.Sysinfo(si)
	if err != nil {
		return cgroupLimit
	}

	//nolint:unconvert
	physicalLimit := uint64(si.Totalram)
	if cgroupLimit > physicalLimit {
		return physicalLimit
	}
	return cgroupLimit
}

func calculateCPUPercent(stats *cgroups.Metrics, previousCPU, previousSystem uint64) float64 {
	var (
		cpuPercent  = 0.0
		cpuDelta    = float64(stats.CPU.Usage.Total - previousCPU)
		systemDelta = float64(uint64(time.Now().UnixNano()) - previousSystem)
	)
	if systemDelta > 0.0 && cpuDelta > 0.0 {
		// gets a ratio of container cpu usage total, multiplies it by the number of cores (4 cores running
		// at 100% utilization should be 400% utilization), and multiplies that by 100 to get a percentage
		nCPUS := len(stats.CPU.Usage.PerCPU)
		if nCPUS == 0 {
			nCPUS = runtime.NumCPU()
		}
		cpuPercent = (cpuDelta / systemDelta) * float64(nCPUS) * 100
	}
	return cpuPercent
}

func calculateBlockIO(stats *cgroups.Metrics) (read uint64, write uint64) {
	for _, blkIOEntry := range stats.Blkio.IoServiceBytesRecursive {
		switch strings.ToLower(blkIOEntry.Op) {
		case "read":
			read += blkIOEntry.Value
		case "write":
			write += blkIOEntry.Value
		}
	}
	return
}
