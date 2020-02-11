package libpod

// ContainerStats contains the statistics information for a running container
type ContainerStats struct {
	ContainerID   string
	Name          string
	PerCPU        []uint64
	CPU           float64
	CPUNano       uint64
	CPUSystemNano uint64
	SystemNano    uint64
	MemUsage      uint64
	MemLimit      uint64
	MemPerc       float64
	NetInput      uint64
	NetOutput     uint64
	BlockInput    uint64
	BlockOutput   uint64
	PIDs          uint64
}
