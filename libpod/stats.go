// +build linux

package libpod

import (
	"math"
	"strings"
	"syscall"
	"time"

	"github.com/containers/podman/v3/libpod/define"
	"github.com/containers/podman/v3/pkg/cgroups"
	"github.com/pkg/errors"
)

// GetContainerStats gets the running stats for a given container
func (c *Container) GetContainerStats(previousStats *define.ContainerStats) (*define.ContainerStats, error) {
	stats := new(define.ContainerStats)
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

	if c.state.State != define.ContainerStateRunning && c.state.State != define.ContainerStatePaused {
		return stats, define.ErrCtrStateInvalid
	}

	cgroupPath, err := c.cGroupPath()
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

	// If the current total usage in the cgroup is less than what was previously
	// recorded then it means the container was restarted and runs in a new cgroup
	if previousStats.Duration > cgroupStats.CPU.Usage.Total {
		previousStats = &define.ContainerStats{}
	}

	previousCPU := previousStats.CPUNano
	now := uint64(time.Now().UnixNano())
	stats.Duration = cgroupStats.CPU.Usage.Total
	stats.UpTime = time.Duration(stats.Duration)
	stats.CPU = calculateCPUPercent(cgroupStats, previousCPU, now, previousStats.SystemNano)
	stats.AvgCPU = calculateAvgCPU(stats.CPU, previousStats.AvgCPU, previousStats.DataPoints)
	stats.DataPoints = previousStats.DataPoints + 1
	stats.MemUsage = cgroupStats.Memory.Usage.Usage
	stats.MemLimit = c.getMemLimit()
	stats.MemPerc = (float64(stats.MemUsage) / float64(stats.MemLimit)) * 100
	stats.PIDs = 0
	if conState == define.ContainerStateRunning || conState == define.ContainerStatePaused {
		stats.PIDs = cgroupStats.Pids.Current
	}
	stats.BlockInput, stats.BlockOutput = calculateBlockIO(cgroupStats)
	stats.CPUNano = cgroupStats.CPU.Usage.Total
	stats.CPUSystemNano = cgroupStats.CPU.Usage.Kernel
	stats.SystemNano = now
	stats.PerCPU = cgroupStats.CPU.Usage.PerCPU
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

// getMemory limit returns the memory limit for a container
func (c *Container) getMemLimit() uint64 {
	memLimit := uint64(math.MaxUint64)

	if c.config.Spec.Linux != nil && c.config.Spec.Linux.Resources != nil &&
		c.config.Spec.Linux.Resources.Memory != nil && c.config.Spec.Linux.Resources.Memory.Limit != nil {
		memLimit = uint64(*c.config.Spec.Linux.Resources.Memory.Limit)
	}

	si := &syscall.Sysinfo_t{}
	err := syscall.Sysinfo(si)
	if err != nil {
		return memLimit
	}

	//nolint:unconvert
	physicalLimit := uint64(si.Totalram)

	if memLimit <= 0 || memLimit > physicalLimit {
		return physicalLimit
	}

	return memLimit
}

// calculateCPUPercent calculates the cpu usage using the latest measurement in stats.
// previousCPU is the last value of stats.CPU.Usage.Total measured at the time previousSystem.
//  (now - previousSystem) is the time delta in nanoseconds, between the measurement in previousCPU
// and the updated value in stats.
func calculateCPUPercent(stats *cgroups.Metrics, previousCPU, now, previousSystem uint64) float64 {
	var (
		cpuPercent  = 0.0
		cpuDelta    = float64(stats.CPU.Usage.Total - previousCPU)
		systemDelta = float64(now - previousSystem)
	)
	if systemDelta > 0.0 && cpuDelta > 0.0 {
		// gets a ratio of container cpu usage total, and multiplies that by 100 to get a percentage
		cpuPercent = (cpuDelta / systemDelta) * 100
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

// calculateAvgCPU calculates the avg CPU percentage given the previous average and the number of data points.
func calculateAvgCPU(statsCPU float64, prevAvg float64, prevData int64) float64 {
	avgPer := ((prevAvg * float64(prevData)) + statsCPU) / (float64(prevData) + 1)
	return avgPer
}
