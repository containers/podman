package entities

import (
	"errors"
	"strings"
	"time"

	commonFlag "github.com/containers/common/pkg/flag"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/specgen"
	"github.com/containers/podman/v4/pkg/util"
	"github.com/opencontainers/runtime-spec/specs-go"
)

type PodKillOptions struct {
	All    bool
	Latest bool
	Signal string
}

type PodKillReport struct {
	Errs []error
	Id   string //nolint
}

type ListPodsReport struct {
	Cgroup     string
	Containers []*ListPodContainer
	Created    time.Time
	Id         string //nolint
	InfraId    string //nolint
	Name       string
	Namespace  string
	// Network names connected to infra container
	Networks []string
	Status   string
	Labels   map[string]string
}

type ListPodContainer struct {
	Id     string //nolint
	Names  string
	Status string
}

type PodPauseOptions struct {
	All    bool
	Latest bool
}

type PodPauseReport struct {
	Errs []error
	Id   string //nolint
}

type PodunpauseOptions struct {
	All    bool
	Latest bool
}

type PodUnpauseReport struct {
	Errs []error
	Id   string //nolint
}

type PodStopOptions struct {
	All     bool
	Ignore  bool
	Latest  bool
	Timeout int
}

type PodStopReport struct {
	Errs []error
	Id   string //nolint
}

type PodRestartOptions struct {
	All    bool
	Latest bool
}

type PodRestartReport struct {
	Errs []error
	Id   string //nolint
}

type PodStartOptions struct {
	All    bool
	Latest bool
}

type PodStartReport struct {
	Errs []error
	Id   string //nolint
}

type PodRmOptions struct {
	All     bool
	Force   bool
	Ignore  bool
	Latest  bool
	Timeout *uint
}

type PodRmReport struct {
	Err error
	Id  string //nolint
}

// PddSpec is an abstracted version of PodSpecGen designed to eventually accept options
// not meant to be in a specgen
type PodSpec struct {
	PodSpecGen specgen.PodSpecGenerator
}

// PodCreateOptions provides all possible options for creating a pod and its infra container.
// The JSON tags below are made to match the respective field in ContainerCreateOptions for the purpose of mapping.
// swagger:model PodCreateOptions
type PodCreateOptions struct {
	CgroupParent       string            `json:"cgroup_parent,omitempty"`
	CreateCommand      []string          `json:"create_command,omitempty"`
	Devices            []string          `json:"devices,omitempty"`
	DeviceReadBPs      []string          `json:"device_read_bps,omitempty"`
	Hostname           string            `json:"hostname,omitempty"`
	Infra              bool              `json:"infra,omitempty"`
	InfraImage         string            `json:"infra_image,omitempty"`
	InfraName          string            `json:"container_name,omitempty"`
	InfraCommand       *string           `json:"container_command,omitempty"`
	InfraConmonPidFile string            `json:"container_conmon_pidfile,omitempty"`
	Labels             map[string]string `json:"labels,omitempty"`
	Name               string            `json:"name,omitempty"`
	Net                *NetOptions       `json:"net,omitempty"`
	Share              []string          `json:"share,omitempty"`
	ShareParent        *bool             `json:"share_parent,omitempty"`
	Pid                string            `json:"pid,omitempty"`
	Cpus               float64           `json:"cpus,omitempty"`
	CpusetCpus         string            `json:"cpuset_cpus,omitempty"`
	Userns             specgen.Namespace `json:"-"`
	Volume             []string          `json:"volume,omitempty"`
	VolumesFrom        []string          `json:"volumes_from,omitempty"`
	SecurityOpt        []string          `json:"security_opt,omitempty"`
	Sysctl             []string          `json:"sysctl,omitempty"`
}

// PodLogsOptions describes the options to extract pod logs.
type PodLogsOptions struct {
	// Other fields are exactly same as ContainerLogOpts
	ContainerLogsOptions
	// If specified will only fetch the logs of specified container
	ContainerName string
	// Show different colors in the logs.
	Color bool
}

type ContainerCreateOptions struct {
	Annotation        []string
	Attach            []string
	Authfile          string
	BlkIOWeight       string
	BlkIOWeightDevice []string
	CapAdd            []string
	CapDrop           []string
	CgroupNS          string
	CgroupsMode       string
	CgroupParent      string `json:"cgroup_parent,omitempty"`
	CIDFile           string
	ConmonPIDFile     string `json:"container_conmon_pidfile,omitempty"`
	CPUPeriod         uint64
	CPUQuota          int64
	CPURTPeriod       uint64
	CPURTRuntime      int64
	CPUShares         uint64
	CPUS              float64 `json:"cpus,omitempty"`
	CPUSetCPUs        string  `json:"cpuset_cpus,omitempty"`
	CPUSetMems        string
	Devices           []string `json:"devices,omitempty"`
	DeviceCgroupRule  []string
	DeviceReadBPs     []string `json:"device_read_bps,omitempty"`
	DeviceReadIOPs    []string
	DeviceWriteBPs    []string
	DeviceWriteIOPs   []string
	Entrypoint        *string `json:"container_command,omitempty"`
	Env               []string
	EnvHost           bool
	EnvFile           []string
	Expose            []string
	GIDMap            []string
	GroupAdd          []string
	HealthCmd         string
	HealthInterval    string
	HealthRetries     uint
	HealthStartPeriod string
	HealthTimeout     string
	Hostname          string `json:"hostname,omitempty"`
	HTTPProxy         bool
	HostUsers         []string
	ImageVolume       string
	Init              bool
	InitContainerType string
	InitPath          string
	Interactive       bool
	IPC               string
	Label             []string
	LabelFile         []string
	LogDriver         string
	LogOptions        []string
	Memory            string
	MemoryReservation string
	MemorySwap        string
	MemorySwappiness  int64
	Name              string `json:"container_name"`
	NoHealthCheck     bool
	OOMKillDisable    bool
	OOMScoreAdj       *int
	Arch              string
	OS                string
	Variant           string
	PID               string `json:"pid,omitempty"`
	PIDsLimit         *int64
	Platform          string
	Pod               string
	PodIDFile         string
	Personality       string
	PreserveFDs       uint
	Privileged        bool
	PublishAll        bool
	Pull              string
	Quiet             bool
	ReadOnly          bool
	ReadOnlyTmpFS     bool
	Restart           string
	Replace           bool
	Requires          []string
	Rm                bool
	RootFS            bool
	Secrets           []string
	SecurityOpt       []string `json:"security_opt,omitempty"`
	SdNotifyMode      string
	ShmSize           string
	SignaturePolicy   string
	StopSignal        string
	StopTimeout       uint
	StorageOpts       []string
	SubUIDName        string
	SubGIDName        string
	Sysctl            []string `json:"sysctl,omitempty"`
	Systemd           string
	Timeout           uint
	TLSVerify         commonFlag.OptionalBool
	TmpFS             []string
	TTY               bool
	Timezone          string
	Umask             string
	UnsetEnv          []string
	UnsetEnvAll       bool
	UIDMap            []string
	Ulimit            []string
	User              string
	UserNS            string `json:"-"`
	UTS               string
	Mount             []string
	Volume            []string `json:"volume,omitempty"`
	VolumesFrom       []string `json:"volumes_from,omitempty"`
	Workdir           string
	SeccompPolicy     string
	PidFile           string
	ChrootDirs        []string
	IsInfra           bool
	IsClone           bool

	Net *NetOptions `json:"net,omitempty"`

	CgroupConf []string

	PasswdEntry string
}

func NewInfraContainerCreateOptions() ContainerCreateOptions {
	options := ContainerCreateOptions{
		IsInfra:          true,
		ImageVolume:      "bind",
		MemorySwappiness: -1,
	}
	return options
}

type PodCreateReport struct {
	Id string //nolint
}

func (p *PodCreateOptions) CPULimits() *specs.LinuxCPU {
	cpu := &specs.LinuxCPU{}
	hasLimits := false

	if p.Cpus != 0 {
		period, quota := util.CoresToPeriodAndQuota(p.Cpus)
		cpu.Period = &period
		cpu.Quota = &quota
		hasLimits = true
	}
	if p.CpusetCpus != "" {
		cpu.Cpus = p.CpusetCpus
		hasLimits = true
	}
	if !hasLimits {
		return cpu
	}
	return cpu
}

func ToPodSpecGen(s specgen.PodSpecGenerator, p *PodCreateOptions) (*specgen.PodSpecGenerator, error) {
	// Basic Config
	s.Name = p.Name
	s.InfraName = p.InfraName
	out, err := specgen.ParseNamespace(p.Pid)
	if err != nil {
		return nil, err
	}
	s.Pid = out
	s.Hostname = p.Hostname
	s.Labels = p.Labels
	s.Devices = p.Devices
	s.SecurityOpt = p.SecurityOpt
	s.NoInfra = !p.Infra
	if p.InfraCommand != nil && len(*p.InfraCommand) > 0 {
		s.InfraCommand = strings.Split(*p.InfraCommand, " ")
	}
	if len(p.InfraConmonPidFile) > 0 {
		s.InfraConmonPidFile = p.InfraConmonPidFile
	}
	s.InfraImage = p.InfraImage
	s.SharedNamespaces = p.Share
	s.ShareParent = p.ShareParent
	s.PodCreateCommand = p.CreateCommand
	s.VolumesFrom = p.VolumesFrom

	// Networking config

	if p.Net != nil {
		s.NetNS = p.Net.Network
		s.PortMappings = p.Net.PublishPorts
		s.Networks = p.Net.Networks
		s.NetworkOptions = p.Net.NetworkOptions
		if p.Net.UseImageResolvConf {
			s.NoManageResolvConf = true
		}
		s.DNSServer = p.Net.DNSServers
		s.DNSSearch = p.Net.DNSSearch
		s.DNSOption = p.Net.DNSOptions
		s.NoManageHosts = p.Net.NoHosts
		s.HostAdd = p.Net.AddHosts
	}

	// Cgroup
	s.CgroupParent = p.CgroupParent

	// Resource config
	cpuDat := p.CPULimits()
	if s.ResourceLimits == nil {
		s.ResourceLimits = &specs.LinuxResources{}
		s.ResourceLimits.CPU = &specs.LinuxCPU{}
	}
	if cpuDat != nil {
		s.ResourceLimits.CPU = cpuDat
		if p.Cpus != 0 {
			s.CPUPeriod = *cpuDat.Period
			s.CPUQuota = *cpuDat.Quota
		}
	}
	s.Userns = p.Userns
	sysctl := map[string]string{}
	if ctl := p.Sysctl; len(ctl) > 0 {
		sysctl, err = util.ValidateSysctls(ctl)
		if err != nil {
			return nil, err
		}
	}
	s.Sysctl = sysctl

	return &s, nil
}

type PodPruneOptions struct {
	Force bool `json:"force" schema:"force"`
}

type PodPruneReport struct {
	Err error
	Id  string //nolint
}

type PodTopOptions struct {
	// CLI flags.
	ListDescriptors bool
	Latest          bool

	// Options for the API.
	Descriptors []string
	NameOrID    string
}

type PodPSOptions struct {
	CtrNames  bool
	CtrIds    bool
	CtrStatus bool
	Filters   map[string][]string
	Format    string
	Latest    bool
	Namespace bool
	Quiet     bool
	Sort      string
}

type PodInspectOptions struct {
	Latest bool

	// Options for the API.
	NameOrID string

	Format string
}

type PodInspectReport struct {
	*define.InspectPodData
}

// PodStatsOptions are options for the pod stats command.
type PodStatsOptions struct {
	// All - provide stats for all running pods.
	All bool
	// Latest - provide stats for the latest pod.
	Latest bool
}

// PodStatsReport includes pod-resource statistics data.
type PodStatsReport struct {
	CPU           string
	MemUsage      string
	MemUsageBytes string
	Mem           string
	NetIO         string
	BlockIO       string
	PIDS          string
	Pod           string
	CID           string
	Name          string
}

// ValidatePodStatsOptions validates the specified slice and options. Allows
// for sharing code in the front- and the back-end.
func ValidatePodStatsOptions(args []string, options *PodStatsOptions) error {
	num := 0
	if len(args) > 0 {
		num++
	}
	if options.All {
		num++
	}
	if options.Latest {
		num++
	}
	switch num {
	case 0:
		// Podman v1 compat: if nothing's specified get all running
		// pods.
		options.All = true
		return nil
	case 1:
		return nil
	default:
		return errors.New("--all, --latest and arguments cannot be used together")
	}
}

// Converts PodLogOptions to ContainerLogOptions
func PodLogsOptionsToContainerLogsOptions(options PodLogsOptions) ContainerLogsOptions {
	// PodLogsOptions are similar but contains few extra fields like ctrName
	// So cast other values as is so we can re-use the code
	containerLogsOpts := ContainerLogsOptions{
		Details:      options.Details,
		Latest:       options.Latest,
		Follow:       options.Follow,
		Names:        options.Names,
		Since:        options.Since,
		Until:        options.Until,
		Tail:         options.Tail,
		Timestamps:   options.Timestamps,
		Colors:       options.Colors,
		StdoutWriter: options.StdoutWriter,
		StderrWriter: options.StderrWriter,
	}
	return containerLogsOpts
}
