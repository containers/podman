package main

import (
	"encoding/json"

	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/projectatomic/libpod/cmd/kpod/formats"
	"github.com/projectatomic/libpod/libpod"
	"github.com/urfave/cli"
)

const (
	inspectTypeContainer = "container"
	inspectTypeImage     = "image"
	inspectAll           = "all"
)

var (
	inspectFlags = []cli.Flag{
		cli.StringFlag{
			Name:  "type, t",
			Value: inspectAll,
			Usage: "Return JSON for specified type, (e.g image, container or task)",
		},
		cli.StringFlag{
			Name:  "format, f",
			Usage: "Change the output format to a Go template",
		},
		cli.BoolFlag{
			Name:  "size",
			Usage: "Display total file size if the type is container",
		},
	}
	inspectDescription = "This displays the low-level information on containers and images identified by name or ID. By default, this will render all results in a JSON array. If the container and image have the same name, this will return container JSON for unspecified type."
	inspectCommand     = cli.Command{
		Name:        "inspect",
		Usage:       "Displays the configuration of a container or image",
		Description: inspectDescription,
		Flags:       inspectFlags,
		Action:      inspectCmd,
		ArgsUsage:   "CONTAINER-OR-IMAGE",
	}
)

func inspectCmd(c *cli.Context) error {
	args := c.Args()
	if len(args) == 0 {
		return errors.Errorf("container or image name must be specified: kpod inspect [options [...]] name")
	}
	if len(args) > 1 {
		return errors.Errorf("too many arguments specified")
	}
	if err := validateFlags(c, inspectFlags); err != nil {
		return err
	}

	runtime, err := getRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}
	defer runtime.Shutdown(false)

	if c.String("type") != inspectTypeContainer && c.String("type") != inspectTypeImage && c.String("type") != inspectAll {
		return errors.Errorf("the only recognized types are %q, %q, and %q", inspectTypeContainer, inspectTypeImage, inspectAll)
	}

	name := args[0]

	outputFormat := c.String("format")
	var data interface{}
	switch c.String("type") {
	case inspectTypeContainer:
		ctr, err := runtime.LookupContainer(name)
		if err != nil {
			return errors.Wrapf(err, "error looking up container %q", name)
		}
		libpodInspectData, err := ctr.Inspect(c.Bool("size"))
		if err != nil {
			return errors.Wrapf(err, "error getting libpod container inspect data %q", ctr.ID)
		}
		data, err = getCtrInspectInfo(ctr, libpodInspectData)
		if err != nil {
			return errors.Wrapf(err, "error parsing container data %q", ctr.ID())
		}
	case inspectTypeImage:
		image, err := runtime.GetImage(name)
		if err != nil {
			return errors.Wrapf(err, "error getting image %q", name)
		}
		data, err = runtime.GetImageInspectInfo(*image)
		if err != nil {
			return errors.Wrapf(err, "error parsing image data %q", image.ID)
		}
	case inspectAll:
		ctr, err := runtime.LookupContainer(name)
		if err != nil {
			image, err := runtime.GetImage(name)
			if err != nil {
				return errors.Wrapf(err, "error getting image %q", name)
			}
			data, err = runtime.GetImageInspectInfo(*image)
			if err != nil {
				return errors.Wrapf(err, "error parsing image data %q", image.ID)
			}
		} else {
			libpodInspectData, err := ctr.Inspect(c.Bool("size"))
			if err != nil {
				return errors.Wrapf(err, "error getting libpod container inspect data %q", ctr.ID)
			}
			data, err = getCtrInspectInfo(ctr, libpodInspectData)
			if err != nil {
				return errors.Wrapf(err, "error parsing container data %q", ctr.ID)
			}
		}
	}

	var out formats.Writer
	if outputFormat != "" && outputFormat != formats.JSONString {
		//template
		out = formats.StdoutTemplate{Output: data, Template: outputFormat}
	} else {
		// default is json output
		out = formats.JSONStruct{Output: data}
	}

	formats.Writer(out).Out()
	return nil
}

func getCtrInspectInfo(ctr *libpod.Container, ctrInspectData *libpod.ContainerInspectData) (*ContainerData, error) {
	config := ctr.Config()
	spec := config.Spec

	cpus, mems, period, quota, realtimePeriod, realtimeRuntime, shares := getCPUInfo(spec)
	blkioWeight, blkioWeightDevice, blkioReadBps, blkioWriteBps, blkioReadIOPS, blkioeWriteIOPS := getBLKIOInfo(spec)
	memKernel, memReservation, memSwap, memSwappiness, memDisableOOMKiller := getMemoryInfo(spec)
	pidsLimit := getPidsInfo(spec)
	cgroup := getCgroup(spec)

	artifact, err := ctr.GetArtifact("create-config")
	if err != nil {
		return nil, errors.Wrapf(err, "error getting artifact %q", ctr.ID())
	}
	var createArtifact createConfig
	if err := json.Unmarshal(artifact, &createArtifact); err != nil {
		return nil, err
	}

	data := &ContainerData{
		CtrInspectData: ctrInspectData,
		HostConfig: &HostConfig{
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
			CPUSetCpus:           cpus,
			CPUSetMems:           mems,
			Devices:              spec.Linux.Devices,
			KernelMemory:         memKernel,
			MemoryReservation:    memReservation,
			MemorySwap:           memSwap,
			MemorySwappiness:     memSwappiness,
			OomKillDisable:       memDisableOOMKiller,
			PidsLimit:            pidsLimit,
			Privileged:           spec.Process.NoNewPrivileges,
			ReadonlyRootfs:       spec.Root.Readonly,
			Runtime:              ctr.RuntimeName(),
			NetworkMode:          string(createArtifact.NetMode),
			IpcMode:              string(createArtifact.IpcMode),
			Cgroup:               cgroup,
			UTSMode:              string(createArtifact.UtsMode),
			UsernsMode:           createArtifact.NsUser,
			GroupAdd:             spec.Process.User.AdditionalGids,
			ContainerIDFile:      createArtifact.CidFile,
			AutoRemove:           createArtifact.Rm,
			CapAdd:               createArtifact.CapAdd,
			CapDrop:              createArtifact.CapDrop,
			DNS:                  createArtifact.DnsServers,
			DNSOptions:           createArtifact.DnsOpt,
			DNSSearch:            createArtifact.DnsSearch,
			PidMode:              string(createArtifact.PidMode),
			CgroupParent:         createArtifact.CgroupParent,
			ShmSize:              createArtifact.Resources.ShmSize,
			Memory:               createArtifact.Resources.Memory,
			Ulimits:              createArtifact.Resources.Ulimit,
			SecurityOpt:          createArtifact.SecurityOpts,
		},
		Config: &CtrConfig{
			Hostname:    spec.Hostname,
			User:        spec.Process.User,
			Env:         spec.Process.Env,
			Image:       config.RootfsImageName,
			WorkingDir:  spec.Process.Cwd,
			Labels:      config.Labels,
			Annotations: spec.Annotations,
			Tty:         spec.Process.Terminal,
			OpenStdin:   config.Stdin,
			StopSignal:  config.StopSignal,
			Cmd:         config.Spec.Process.Args,
			Entrypoint:  createArtifact.Entrypoint,
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

// ContainerData holds the kpod inspect data for a container
type ContainerData struct {
	CtrInspectData *libpod.ContainerInspectData `json:"CtrInspectData"`
	HostConfig     *HostConfig                  `json:"HostConfig"`
	Config         *CtrConfig                   `json:"Config"`
}

// LogConfig holds the log information for a container
type LogConfig struct {
	Type   string            `json:"Type"`   // TODO
	Config map[string]string `json:"Config"` //idk type, TODO
}

// HostConfig represents the host configuration for the container
type HostConfig struct {
	ContainerIDFile      string                      `json:"ContainerIDFile"`
	LogConfig            *LogConfig                  `json:"LogConfig"` //TODO
	NetworkMode          string                      `json:"NetworkMode"`
	PortBindings         map[string]struct{}         `json:"PortBindings"` //TODO
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
	ReadonlyRootfs       bool                        `json:"ReadonlyRootfs"`
	SecurityOpt          []string                    `json:"SecurityOpt"`
	UTSMode              string                      `json:"UTSMode"`
	UsernsMode           string                      `json:"UsernsMode"`
	ShmSize              string                      `json:"ShmSize"`
	Runtime              string                      `json:"Runtime"`
	ConsoleSize          *specs.Box                  `json:"ConsoleSize"`
	Isolation            string                      `json:"Isolation"` //TODO
	CPUShares            *uint64                     `json:"CPUSShares"`
	Memory               int64                       `json:"Memory"`
	NanoCpus             int                         `json:"NanoCpus"` //check type, TODO
	CgroupParent         string                      `json:"CgroupParent"`
	BlkioWeight          *uint16                     `json:"BlkioWeight"`
	BlkioWeightDevice    []specs.LinuxWeightDevice   `json:"BlkioWeightDevice"`
	BlkioDeviceReadBps   []specs.LinuxThrottleDevice `json:"BlkioDeviceReadBps"`
	BlkioDeviceWriteBps  []specs.LinuxThrottleDevice `json:"BlkioDeviceWriteBps"`
	BlkioDeviceReadIOps  []specs.LinuxThrottleDevice `json:"BlkioDeviceReadIOps"`
	BlkioDeviceWriteIOps []specs.LinuxThrottleDevice `json:"BlkioDeviceWriteIOps"`
	CPUPeriod            *uint64                     `json:"CPUPeriod"`
	CPUQuota             *int64                      `json:"CPUQuota"`
	CPURealtimePeriod    *uint64                     `json:"CPURealtimePeriod"`
	CPURealtimeRuntime   *int64                      `json:"CPURealtimeRuntime"`
	CPUSetCpus           string                      `json:"CPUSetCpus"`
	CPUSetMems           string                      `json:"CPUSetMems"`
	Devices              []specs.LinuxDevice         `json:"Devices"`
	DiskQuota            int                         `json:"DiskQuota"` //check type, TODO
	KernelMemory         *int64                      `json:"KernelMemory"`
	MemoryReservation    *int64                      `json:"MemoryReservation"`
	MemorySwap           *int64                      `json:"MemorySwap"`
	MemorySwappiness     *uint64                     `json:"MemorySwappiness"`
	OomKillDisable       *bool                       `json:"OomKillDisable"`
	PidsLimit            *int64                      `json:"PidsLimit"`
	Ulimits              []string                    `json:"Ulimits"`
	CPUCount             int                         `json:"CPUCount"`           //check type, TODO
	CPUPercent           int                         `json:"CPUPercent"`         //check type, TODO
	IOMaximumIOps        int                         `json:"IOMaximumIOps"`      //check type, TODO
	IOMaximumBandwidth   int                         `json:"IOMaximumBandwidth"` //check type, TODO
}

// CtrConfig holds information about the container configuration
type CtrConfig struct {
	Hostname     string              `json:"Hostname"`
	DomainName   string              `json:"Domainname"` //TODO
	User         specs.User          `json:"User"`
	AttachStdin  bool                `json:"AttachStdin"`  //TODO
	AttachStdout bool                `json:"AttachStdout"` //TODO
	AttachStderr bool                `json:"AttachStderr"` //TODO
	Tty          bool                `json:"Tty"`
	OpenStdin    bool                `json:"OpenStdin"`
	StdinOnce    bool                `json:"StdinOnce"` //TODO
	Env          []string            `json:"Env"`
	Cmd          []string            `json:"Cmd"`
	Image        string              `json:"Image"`
	Volumes      map[string]struct{} `json:"Volumes"`
	WorkingDir   string              `json:"WorkingDir"`
	Entrypoint   string              `json:"Entrypoint"`
	Labels       map[string]string   `json:"Labels"`
	Annotations  map[string]string   `json:"Annotations"`
	StopSignal   uint                `json:"StopSignal"`
}
