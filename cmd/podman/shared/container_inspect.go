package shared

import (
	"github.com/containers/libpod/libpod"
	cc "github.com/containers/libpod/pkg/spec"
	"github.com/docker/go-connections/nat"
	"github.com/opencontainers/runtime-spec/specs-go"
)

// InspectContainer holds all inspect data for a container.
// The format of individual components is fixed so the overall structure, when
// JSON encoded, matches the output of `docker inspect`.
// It combines Libpod-source inspect data with Podman-specific inspect data.
type InspectContainer struct {
	*libpod.InspectContainerData
	HostConfig *InspectContainerHostConfig `json:"HostConfig"`
}

// InspectContainerHostConfig holds Container configuration that is not specific
// to Libpod. This information is (mostly) stored by Podman as an artifact.
// This struct is matched to the output of `docker inspect`.
type InspectContainerHostConfig struct {
	ContainerIDFile      string                      `json:"ContainerIDFile"`
	LogConfig            *InspectLogConfig           `json:"LogConfig"` //TODO
	NetworkMode          string                      `json:"NetworkMode"`
	PortBindings         nat.PortMap                 `json:"PortBindings"` //TODO
	AutoRemove           bool                        `json:"AutoRemove"`
	CapAdd               []string                    `json:"CapAdd"`
	CapDrop              []string                    `json:"CapDrop"`
	DNS                  []string                    `json:"DNS"`
	DNSOptions           []string                    `json:"DNSOptions"`
	DNSSearch            []string                    `json:"DNSSearch"`
	ExtraHosts           []string                    `json:"ExtraHosts"`
	GroupAdd             []uint32                    `json:"GroupAdd"`
	IpcMode              string                      `json:"IpcMode"`
	Cgroup               string                      `json:"Cgroup"`
	OomScoreAdj          *int                        `json:"OomScoreAdj"`
	PidMode              string                      `json:"PidMode"`
	Privileged           bool                        `json:"Privileged"`
	PublishAllPorts      bool                        `json:"PublishAllPorts"` //TODO
	ReadOnlyRootfs       bool                        `json:"ReadonlyRootfs"`
	ReadOnlyTmpfs        bool                        `json:"ReadonlyTmpfs"`
	SecurityOpt          []string                    `json:"SecurityOpt"`
	UTSMode              string                      `json:"UTSMode"`
	UsernsMode           string                      `json:"UsernsMode"`
	ShmSize              int64                       `json:"ShmSize"`
	Runtime              string                      `json:"Runtime"`
	ConsoleSize          *specs.Box                  `json:"ConsoleSize"`
	CPUShares            *uint64                     `json:"CpuShares"`
	Memory               int64                       `json:"Memory"`
	NanoCPUs             int                         `json:"NanoCpus"`
	CgroupParent         string                      `json:"CgroupParent"`
	BlkioWeight          *uint16                     `json:"BlkioWeight"`
	BlkioWeightDevice    []specs.LinuxWeightDevice   `json:"BlkioWeightDevice"`
	BlkioDeviceReadBps   []specs.LinuxThrottleDevice `json:"BlkioDeviceReadBps"`
	BlkioDeviceWriteBps  []specs.LinuxThrottleDevice `json:"BlkioDeviceWriteBps"`
	BlkioDeviceReadIOps  []specs.LinuxThrottleDevice `json:"BlkioDeviceReadIOps"`
	BlkioDeviceWriteIOps []specs.LinuxThrottleDevice `json:"BlkioDeviceWriteIOps"`
	CPUPeriod            *uint64                     `json:"CpuPeriod"`
	CPUQuota             *int64                      `json:"CpuQuota"`
	CPURealtimePeriod    *uint64                     `json:"CpuRealtimePeriod"`
	CPURealtimeRuntime   *int64                      `json:"CpuRealtimeRuntime"`
	CPUSetCPUs           string                      `json:"CpuSetCpus"`
	CPUSetMems           string                      `json:"CpuSetMems"`
	Devices              []specs.LinuxDevice         `json:"Devices"`
	DiskQuota            int                         `json:"DiskQuota"` //check type, TODO
	KernelMemory         *int64                      `json:"KernelMemory"`
	MemoryReservation    *int64                      `json:"MemoryReservation"`
	MemorySwap           *int64                      `json:"MemorySwap"`
	MemorySwappiness     *uint64                     `json:"MemorySwappiness"`
	OomKillDisable       *bool                       `json:"OomKillDisable"`
	PidsLimit            *int64                      `json:"PidsLimit"`
	Ulimits              []string                    `json:"Ulimits"`
	CPUCount             int                         `json:"CpuCount"`
	CPUPercent           int                         `json:"CpuPercent"`
	IOMaximumIOps        int                         `json:"IOMaximumIOps"`      //check type, TODO
	IOMaximumBandwidth   int                         `json:"IOMaximumBandwidth"` //check type, TODO
	Tmpfs                []string                    `json:"Tmpfs"`
}

// InspectLogConfig holds information about a container's configured log driver
// and is presently unused. It is retained for Docker compatibility.
type InspectLogConfig struct {
	Type   string            `json:"Type"`
	Config map[string]string `json:"Config"` //idk type, TODO
}

// GetCtrInspectInfo inspects a container, combining Libpod inspect information
// with other information not stored in Libpod and returning a struct that, when
// formatted for JSON output, is compatible with `docker inspect`.
func GetCtrInspectInfo(config *libpod.ContainerConfig, ctrInspectData *libpod.InspectContainerData, createArtifact *cc.CreateConfig) (*InspectContainer, error) {
	spec := config.Spec

	cpus, mems, period, quota, realtimePeriod, realtimeRuntime, shares := getCPUInfo(spec)
	blkioWeight, blkioWeightDevice, blkioReadBps, blkioWriteBps, blkioReadIOPS, blkioeWriteIOPS := getBLKIOInfo(spec)
	memKernel, memReservation, memSwap, memSwappiness, memDisableOOMKiller := getMemoryInfo(spec)
	pidsLimit := getPidsInfo(spec)
	cgroup := getCgroup(spec)
	logConfig := InspectLogConfig{
		config.LogDriver,
		make(map[string]string),
	}

	data := &InspectContainer{
		ctrInspectData,
		&InspectContainerHostConfig{
			ConsoleSize:          spec.Process.ConsoleSize,
			OomScoreAdj:          spec.Process.OOMScoreAdj,
			CPUShares:            shares,
			BlkioWeight:          blkioWeight,
			BlkioWeightDevice:    blkioWeightDevice,
			BlkioDeviceReadBps:   blkioReadBps,
			BlkioDeviceWriteBps:  blkioWriteBps,
			BlkioDeviceReadIOps:  blkioReadIOPS,
			BlkioDeviceWriteIOps: blkioeWriteIOPS,
			CPUPeriod:            period,
			CPUQuota:             quota,
			CPURealtimePeriod:    realtimePeriod,
			CPURealtimeRuntime:   realtimeRuntime,
			CPUSetCPUs:           cpus,
			CPUSetMems:           mems,
			Devices:              spec.Linux.Devices,
			KernelMemory:         memKernel,
			LogConfig:            &logConfig,
			MemoryReservation:    memReservation,
			MemorySwap:           memSwap,
			MemorySwappiness:     memSwappiness,
			OomKillDisable:       memDisableOOMKiller,
			PidsLimit:            pidsLimit,
			Privileged:           config.Privileged,
			ReadOnlyRootfs:       spec.Root.Readonly,
			ReadOnlyTmpfs:        createArtifact.ReadOnlyTmpfs,
			Runtime:              config.OCIRuntime,
			NetworkMode:          string(createArtifact.NetMode),
			IpcMode:              string(createArtifact.IpcMode),
			Cgroup:               cgroup,
			UTSMode:              string(createArtifact.UtsMode),
			UsernsMode:           string(createArtifact.UsernsMode),
			GroupAdd:             spec.Process.User.AdditionalGids,
			ContainerIDFile:      createArtifact.CidFile,
			AutoRemove:           createArtifact.Rm,
			CapAdd:               createArtifact.CapAdd,
			CapDrop:              createArtifact.CapDrop,
			DNS:                  createArtifact.DNSServers,
			DNSOptions:           createArtifact.DNSOpt,
			DNSSearch:            createArtifact.DNSSearch,
			PidMode:              string(createArtifact.PidMode),
			CgroupParent:         createArtifact.CgroupParent,
			ShmSize:              createArtifact.Resources.ShmSize,
			Memory:               createArtifact.Resources.Memory,
			Ulimits:              createArtifact.Resources.Ulimit,
			SecurityOpt:          createArtifact.SecurityOpts,
			Tmpfs:                createArtifact.Tmpfs,
		},
	}
	return data, nil
}

func getCPUInfo(spec *specs.Spec) (string, string, *uint64, *int64, *uint64, *int64, *uint64) {
	if spec.Linux.Resources == nil {
		return "", "", nil, nil, nil, nil, nil
	}
	cpu := spec.Linux.Resources.CPU
	if cpu == nil {
		return "", "", nil, nil, nil, nil, nil
	}
	return cpu.Cpus, cpu.Mems, cpu.Period, cpu.Quota, cpu.RealtimePeriod, cpu.RealtimeRuntime, cpu.Shares
}

func getBLKIOInfo(spec *specs.Spec) (*uint16, []specs.LinuxWeightDevice, []specs.LinuxThrottleDevice, []specs.LinuxThrottleDevice, []specs.LinuxThrottleDevice, []specs.LinuxThrottleDevice) {
	if spec.Linux.Resources == nil {
		return nil, nil, nil, nil, nil, nil
	}
	blkio := spec.Linux.Resources.BlockIO
	if blkio == nil {
		return nil, nil, nil, nil, nil, nil
	}
	return blkio.Weight, blkio.WeightDevice, blkio.ThrottleReadBpsDevice, blkio.ThrottleWriteBpsDevice, blkio.ThrottleReadIOPSDevice, blkio.ThrottleWriteIOPSDevice
}

func getMemoryInfo(spec *specs.Spec) (*int64, *int64, *int64, *uint64, *bool) {
	if spec.Linux.Resources == nil {
		return nil, nil, nil, nil, nil
	}
	memory := spec.Linux.Resources.Memory
	if memory == nil {
		return nil, nil, nil, nil, nil
	}
	return memory.Kernel, memory.Reservation, memory.Swap, memory.Swappiness, memory.DisableOOMKiller
}

func getPidsInfo(spec *specs.Spec) *int64 {
	if spec.Linux.Resources == nil {
		return nil
	}
	pids := spec.Linux.Resources.Pids
	if pids == nil {
		return nil
	}
	return &pids.Limit
}

func getCgroup(spec *specs.Spec) string {
	cgroup := "host"
	for _, ns := range spec.Linux.Namespaces {
		if ns.Type == specs.CgroupNamespace && ns.Path != "" {
			cgroup = "container"
		}
	}
	return cgroup
}
