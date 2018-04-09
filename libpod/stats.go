package libpod

import (
	"strings"
	"syscall"
	"time"

	"github.com/containerd/cgroups"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/cri-o/ocicni/pkg/ocicni"
	"github.com/pkg/errors"
	"github.com/vishvananda/netlink"
)

// ContainerStats contains the statistics information for a running container
type ContainerStats struct {
	ContainerID string
	Name        string
	CPU         float64
	CPUNano     uint64
	SystemNano  uint64
	MemUsage    uint64
	MemLimit    uint64
	MemPerc     float64
	NetInput    uint64
	NetOutput   uint64
	BlockInput  uint64
	BlockOutput uint64
	PIDs        uint64
}

// GetContainerStats gets the running stats for a given container
func (c *Container) GetContainerStats(previousStats *ContainerStats) (*ContainerStats, error) {
	stats := new(ContainerStats)
	stats.ContainerID = c.ID()
	stats.Name = c.Name()
	c.lock.Lock()
	defer c.lock.Unlock()
	if err := c.syncContainer(); err != nil {
		return stats, err
	}
	if c.state.State != ContainerStateRunning {
		return stats, nil
	}

	cgroup, err := cgroups.Load(cgroups.V1, c.CGroupPath())
	if err != nil {
		return stats, errors.Wrapf(err, "unable to load cgroup at %+v", c.CGroupPath())
	}

	cgroupStats, err := cgroup.Stat()
	if err != nil {
		return stats, errors.Wrapf(err, "unable to obtain cgroup stats")
	}
	conState := c.state.State
	if err != nil {
		return stats, errors.Wrapf(err, "unable to determine container state")
	}

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
	if conState == ContainerStateRunning {
		stats.PIDs = cgroupStats.Pids.Current
	}
	stats.BlockInput, stats.BlockOutput = calculateBlockIO(cgroupStats)
	stats.CPUNano = cgroupStats.CPU.Usage.Total
	stats.SystemNano = cgroupStats.CPU.Usage.Kernel
	stats.NetInput = netStats.TxBytes
	stats.NetOutput = netStats.RxBytes

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

	physicalLimit := uint64(si.Totalram)
	if cgroupLimit > physicalLimit {
		return physicalLimit
	}
	return cgroupLimit
}

func getContainerNetIO(ctr *Container) (*netlink.LinkStatistics, error) {
	var netStats *netlink.LinkStatistics
	err := ns.WithNetNSPath(ctr.state.NetNS.Path(), func(_ ns.NetNS) error {
		link, err := netlink.LinkByName(ocicni.DefaultInterfaceName)
		if err != nil {
			return err
		}
		netStats = link.Attrs().Statistics
		return nil
	})
	return netStats, err
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
		cpuPercent = (cpuDelta / systemDelta) * float64(len(stats.CPU.Usage.PerCPU)) * 100
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
