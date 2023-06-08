package libpod

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/containers/common/pkg/cgroups"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/rctl"
	"github.com/containers/storage/pkg/system"
	"github.com/sirupsen/logrus"
)

// getPlatformContainerStats gets the platform-specific running stats
// for a given container.  The previousStats is used to correctly
// calculate cpu percentages. You should pass nil if there is no
// previous stat for this container.
func (c *Container) getPlatformContainerStats(stats *define.ContainerStats, previousStats *define.ContainerStats) error {
	now := uint64(time.Now().UnixNano())

	entries, err := rctl.GetRacct("jail:" + c.jailName())
	if err != nil {
		return fmt.Errorf("unable to read accounting for %s: %w", c.jailName(), err)
	}

	// If the current total usage is less than what was previously
	// recorded then it means the container was restarted and runs
	// in a new jail
	if dur, ok := entries["wallclock"]; ok {
		if previousStats.Duration > dur*1000000000 {
			previousStats = &define.ContainerStats{}
		}
	}

	for key, val := range entries {
		switch key {
		case "cputime": // CPU time, in seconds
			stats.CPUNano = val * 1000000000
			stats.AvgCPU = calculateCPUPercent(stats.CPUNano, 0, now, uint64(c.state.StartedTime.UnixNano()))
		case "datasize": // data size, in bytes
		case "stacksize": // stack size, in bytes
		case "coredumpsize": // core dump size, in bytes
		case "memoryuse": // resident set size, in bytes
			stats.MemUsage = val
		case "memorylocked": // locked memory, in bytes
		case "maxproc": // number of processes
			stats.PIDs = val
		case "openfiles": // file descriptor table size
		case "vmemoryuse": // address space limit, in bytes
		case "pseudoterminals": // number of PTYs
		case "swapuse": // swap space that may be reserved or used, in bytes
		case "nthr": // number of threads
		case "msgqqueued": // number of queued SysV messages
		case "msgqsize": // SysV message queue size, in bytes
		case "nmsgq": // number of SysV message queues
		case "nsem": // number of SysV semaphores
		case "nsemop": // number of SysV semaphores modified in a single semop(2) call
		case "nshm": // number of SysV shared memory segments
		case "shmsize": // SysV shared memory size, in bytes
		case "wallclock": // wallclock time, in seconds
			stats.Duration = val * 1000000000
			stats.UpTime = time.Duration(stats.Duration)
		case "pcpu": // %CPU, in percents of a single CPU core
			stats.CPU = float64(val)
		case "readbps": // filesystem reads, in bytes per second
			stats.BlockInput = val
		case "writebps": // filesystem writes, in bytes per second
			stats.BlockOutput = val
		case "readiops": // filesystem reads, in operations per second
		case "writeiops": // filesystem writes, in operations per second
		}
	}
	stats.MemLimit = c.getMemLimit()
	stats.SystemNano = now

	netStats, err := getContainerNetIO(c)
	if err != nil {
		return err
	}

	// Handle case where the container is not in a network namespace
	if netStats != nil {
		stats.NetInput = netStats.RxBytes
		stats.NetOutput = netStats.TxBytes
	} else {
		stats.NetInput = 0
		stats.NetOutput = 0
	}

	return nil
}

// getMemory limit returns the memory limit for a container
func (c *Container) getMemLimit() uint64 {
	memLimit := uint64(math.MaxUint64)

	resources := c.LinuxResources()
	if resources != nil && resources.Memory != nil && resources.Memory.Limit != nil {
		memLimit = uint64(*resources.Memory.Limit)
	}

	mi, err := system.ReadMemInfo()
	if err != nil {
		logrus.Errorf("ReadMemInfo error: %v", err)
		return 0
	}

	//nolint:unconvert
	physicalLimit := uint64(mi.MemTotal)

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
func calculateCPUPercent(currentCPU, previousCPU, now, previousSystem uint64) float64 {
	var (
		cpuPercent  = 0.0
		cpuDelta    = float64(currentCPU - previousCPU)
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

func getOnlineCPUs(container *Container) (int, error) {
	return 0, nil
}
