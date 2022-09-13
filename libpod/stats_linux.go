//go:build linux
// +build linux

package libpod

import (
	"fmt"
	"math"
	"strings"
	"syscall"
	"time"

	runccgroup "github.com/opencontainers/runc/libcontainer/cgroups"

	"github.com/containers/common/pkg/cgroups"
	"github.com/containers/podman/v4/libpod/define"
)

// getPlatformContainerStats gets the platform-specific running stats
// for a given container.  The previousStats is used to correctly
// calculate cpu percentages. You should pass nil if there is no
// previous stat for this container.
func (c *Container) getPlatformContainerStats(stats *define.ContainerStats, previousStats *define.ContainerStats) error {
	if c.config.NoCgroups {
		return fmt.Errorf("cannot run top on container %s as it did not create a cgroup: %w", c.ID(), define.ErrNoCgroups)
	}

	cgroupPath, err := c.cGroupPath()
	if err != nil {
		return err
	}
	cgroup, err := cgroups.Load(cgroupPath)
	if err != nil {
		return fmt.Errorf("unable to load cgroup at %s: %w", cgroupPath, err)
	}

	// Ubuntu does not have swap memory in cgroups because swap is often not enabled.
	cgroupStats, err := cgroup.Stat()
	if err != nil {
		return fmt.Errorf("unable to obtain cgroup stats: %w", err)
	}
	conState := c.state.State
	netStats, err := getContainerNetIO(c)
	if err != nil {
		return err
	}

	// If the current total usage in the cgroup is less than what was previously
	// recorded then it means the container was restarted and runs in a new cgroup
	if previousStats.Duration > cgroupStats.CpuStats.CpuUsage.TotalUsage {
		previousStats = &define.ContainerStats{}
	}

	previousCPU := previousStats.CPUNano
	now := uint64(time.Now().UnixNano())
	stats.Duration = cgroupStats.CpuStats.CpuUsage.TotalUsage
	stats.UpTime = time.Duration(stats.Duration)
	stats.CPU = calculateCPUPercent(cgroupStats, previousCPU, now, previousStats.SystemNano)
	// calc the average cpu usage for the time the container is running
	stats.AvgCPU = calculateCPUPercent(cgroupStats, 0, now, uint64(c.state.StartedTime.UnixNano()))
	stats.MemUsage = cgroupStats.MemoryStats.Usage.Usage
	stats.MemLimit = c.getMemLimit()
	stats.MemPerc = (float64(stats.MemUsage) / float64(stats.MemLimit)) * 100
	stats.PIDs = 0
	if conState == define.ContainerStateRunning || conState == define.ContainerStatePaused {
		stats.PIDs = cgroupStats.PidsStats.Current
	}
	stats.BlockInput, stats.BlockOutput = calculateBlockIO(cgroupStats)
	stats.CPUNano = cgroupStats.CpuStats.CpuUsage.TotalUsage
	stats.CPUSystemNano = cgroupStats.CpuStats.CpuUsage.UsageInKernelmode
	stats.SystemNano = now
	stats.PerCPU = cgroupStats.CpuStats.CpuUsage.PercpuUsage
	// Handle case where the container is not in a network namespace
	if netStats != nil {
		stats.NetInput = netStats.TxBytes
		stats.NetOutput = netStats.RxBytes
	} else {
		stats.NetInput = 0
		stats.NetOutput = 0
	}

	return nil
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
//
//	(now - previousSystem) is the time delta in nanoseconds, between the measurement in previousCPU
//
// and the updated value in stats.
func calculateCPUPercent(stats *runccgroup.Stats, previousCPU, now, previousSystem uint64) float64 {
	var (
		cpuPercent  = 0.0
		cpuDelta    = float64(stats.CpuStats.CpuUsage.TotalUsage - previousCPU)
		systemDelta = float64(now - previousSystem)
	)
	if systemDelta > 0.0 && cpuDelta > 0.0 {
		// gets a ratio of container cpu usage total, and multiplies that by 100 to get a percentage
		cpuPercent = (cpuDelta / systemDelta) * 100
	}
	return cpuPercent
}

func calculateBlockIO(stats *runccgroup.Stats) (read uint64, write uint64) {
	for _, blkIOEntry := range stats.BlkioStats.IoServiceBytesRecursive {
		switch strings.ToLower(blkIOEntry.Op) {
		case "read":
			read += blkIOEntry.Value
		case "write":
			write += blkIOEntry.Value
		}
	}
	return
}
