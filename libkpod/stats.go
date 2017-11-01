package libkpod

import (
	"path/filepath"
	"syscall"
	"time"

	"strings"

	"github.com/kubernetes-incubator/cri-o/oci"
	"github.com/opencontainers/runc/libcontainer"
)

// ContainerStats contains the statistics information for a running container
type ContainerStats struct {
	Container   string
	CPU         float64
	cpuNano     uint64
	systemNano  uint64
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
func (c *ContainerServer) GetContainerStats(ctr *oci.Container, previousStats *ContainerStats) (*ContainerStats, error) {
	previousCPU := previousStats.cpuNano
	previousSystem := previousStats.systemNano
	libcontainerStats, err := c.LibcontainerStats(ctr)
	if err != nil {
		return nil, err
	}
	cgroupStats := libcontainerStats.CgroupStats
	stats := new(ContainerStats)
	stats.Container = ctr.ID()
	stats.CPU = calculateCPUPercent(libcontainerStats, previousCPU, previousSystem)
	stats.MemUsage = cgroupStats.MemoryStats.Usage.Usage
	stats.MemLimit = getMemLimit(cgroupStats.MemoryStats.Usage.Limit)
	stats.MemPerc = float64(stats.MemUsage) / float64(stats.MemLimit)
	stats.PIDs = cgroupStats.PidsStats.Current
	stats.BlockInput, stats.BlockOutput = calculateBlockIO(libcontainerStats)
	stats.NetInput, stats.NetOutput = getContainerNetIO(libcontainerStats)

	return stats, nil
}

func loadFactory(root string) (libcontainer.Factory, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	cgroupManager := libcontainer.Cgroupfs
	return libcontainer.New(abs, cgroupManager, libcontainer.CriuPath(""))
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

// Returns the total number of bytes transmitted and received for the given container stats
func getContainerNetIO(stats *libcontainer.Stats) (received uint64, transmitted uint64) {
	for _, iface := range stats.Interfaces {
		received += iface.RxBytes
		transmitted += iface.TxBytes
	}
	return
}

func calculateCPUPercent(stats *libcontainer.Stats, previousCPU, previousSystem uint64) float64 {
	var (
		cpuPercent  = 0.0
		cpuDelta    = float64(stats.CgroupStats.CpuStats.CpuUsage.TotalUsage - previousCPU)
		systemDelta = float64(uint64(time.Now().UnixNano()) - previousSystem)
	)
	if systemDelta > 0.0 && cpuDelta > 0.0 {
		// gets a ratio of container cpu usage total, multiplies it by the number of cores (4 cores running
		// at 100% utilization should be 400% utilization), and multiplies that by 100 to get a percentage
		cpuPercent = (cpuDelta / systemDelta) * float64(len(stats.CgroupStats.CpuStats.CpuUsage.PercpuUsage)) * 100
	}
	return cpuPercent
}

func calculateBlockIO(stats *libcontainer.Stats) (read uint64, write uint64) {
	for _, blkIOEntry := range stats.CgroupStats.BlkioStats.IoServiceBytesRecursive {
		switch strings.ToLower(blkIOEntry.Op) {
		case "read":
			read += blkIOEntry.Value
		case "write":
			write += blkIOEntry.Value
		}
	}
	return
}
