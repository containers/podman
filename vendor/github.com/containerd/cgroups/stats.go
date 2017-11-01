package cgroups

import "sync"

type Stats struct {
	cpuMu sync.Mutex

	Hugetlb map[string]HugetlbStat
	Pids    *PidsStat
	Cpu     *CpuStat
	Memory  *MemoryStat
	Blkio   *BlkioStat
}

type HugetlbStat struct {
	Usage   uint64
	Max     uint64
	Failcnt uint64
}

type PidsStat struct {
	Current uint64
	Limit   uint64
}

type CpuStat struct {
	Usage      CpuUsage
	Throttling Throttle
}

type CpuUsage struct {
	// Units: nanoseconds.
	Total  uint64
	PerCpu []uint64
	Kernel uint64
	User   uint64
}

type Throttle struct {
	Periods          uint64
	ThrottledPeriods uint64
	ThrottledTime    uint64
}

type MemoryStat struct {
	Cache                   uint64
	RSS                     uint64
	RSSHuge                 uint64
	MappedFile              uint64
	Dirty                   uint64
	Writeback               uint64
	PgPgIn                  uint64
	PgPgOut                 uint64
	PgFault                 uint64
	PgMajFault              uint64
	InactiveAnon            uint64
	ActiveAnon              uint64
	InactiveFile            uint64
	ActiveFile              uint64
	Unevictable             uint64
	HierarchicalMemoryLimit uint64
	HierarchicalSwapLimit   uint64
	TotalCache              uint64
	TotalRSS                uint64
	TotalRSSHuge            uint64
	TotalMappedFile         uint64
	TotalDirty              uint64
	TotalWriteback          uint64
	TotalPgPgIn             uint64
	TotalPgPgOut            uint64
	TotalPgFault            uint64
	TotalPgMajFault         uint64
	TotalInactiveAnon       uint64
	TotalActiveAnon         uint64
	TotalInactiveFile       uint64
	TotalActiveFile         uint64
	TotalUnevictable        uint64

	Usage     MemoryEntry
	Swap      MemoryEntry
	Kernel    MemoryEntry
	KernelTCP MemoryEntry
}

type MemoryEntry struct {
	Limit   uint64
	Usage   uint64
	Max     uint64
	Failcnt uint64
}

type BlkioStat struct {
	IoServiceBytesRecursive []BlkioEntry
	IoServicedRecursive     []BlkioEntry
	IoQueuedRecursive       []BlkioEntry
	IoServiceTimeRecursive  []BlkioEntry
	IoWaitTimeRecursive     []BlkioEntry
	IoMergedRecursive       []BlkioEntry
	IoTimeRecursive         []BlkioEntry
	SectorsRecursive        []BlkioEntry
}

type BlkioEntry struct {
	Op     string
	Device string
	Major  uint64
	Minor  uint64
	Value  uint64
}
