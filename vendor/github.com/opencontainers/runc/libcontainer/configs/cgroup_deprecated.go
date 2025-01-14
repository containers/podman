package configs // Deprecated: use [github.com/opencontainers/runc/libcontainer/cgroups].

import "github.com/opencontainers/runc/libcontainer/cgroups"

type (
	Cgroup         = cgroups.Cgroup
	Resources      = cgroups.Resources
	FreezerState   = cgroups.FreezerState
	LinuxRdma      = cgroups.LinuxRdma
	BlockIODevice  = cgroups.BlockIODevice
	WeightDevice   = cgroups.WeightDevice
	ThrottleDevice = cgroups.ThrottleDevice
	HugepageLimit  = cgroups.HugepageLimit
	IfPrioMap      = cgroups.IfPrioMap
)

const (
	Undefined = cgroups.Undefined
	Frozen    = cgroups.Frozen
	Thawed    = cgroups.Thawed
)

var (
	NewWeightDevice   = cgroups.NewWeightDevice
	NewThrottleDevice = cgroups.NewThrottleDevice
)
