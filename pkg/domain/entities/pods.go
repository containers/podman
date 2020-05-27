package entities

import (
	"errors"
	"strings"
	"time"

	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/specgen"
)

type PodKillOptions struct {
	All    bool
	Latest bool
	Signal string
}

type PodKillReport struct {
	Errs []error
	Id   string
}

type ListPodsReport struct {
	Cgroup     string
	Containers []*ListPodContainer
	Created    time.Time
	Id         string
	InfraId    string
	Name       string
	Namespace  string
	Status     string
}

type ListPodContainer struct {
	Id     string
	Names  string
	Status string
}

type PodPauseOptions struct {
	All    bool
	Latest bool
}

type PodPauseReport struct {
	Errs []error
	Id   string
}

type PodunpauseOptions struct {
	All    bool
	Latest bool
}

type PodUnpauseReport struct {
	Errs []error
	Id   string
}

type PodStopOptions struct {
	All     bool
	Ignore  bool
	Latest  bool
	Timeout int
}

type PodStopReport struct {
	Errs []error
	Id   string
}

type PodRestartOptions struct {
	All    bool
	Latest bool
}

type PodRestartReport struct {
	Errs []error
	Id   string
}

type PodStartOptions struct {
	All    bool
	Latest bool
}

type PodStartReport struct {
	Errs []error
	Id   string
}

type PodRmOptions struct {
	All    bool
	Force  bool
	Ignore bool
	Latest bool
}

type PodRmReport struct {
	Err error
	Id  string
}

type PodCreateOptions struct {
	CGroupParent string
	Hostname     string
	Infra        bool
	InfraImage   string
	InfraCommand string
	Labels       map[string]string
	Name         string
	Net          *NetOptions
	Share        []string
}

type PodCreateReport struct {
	Id string
}

func (p PodCreateOptions) ToPodSpecGen(s *specgen.PodSpecGenerator) {
	// Basic Config
	s.Name = p.Name
	s.Hostname = p.Hostname
	s.Labels = p.Labels
	s.NoInfra = !p.Infra
	if len(p.InfraCommand) > 0 {
		s.InfraCommand = strings.Split(p.InfraCommand, " ")
	}
	s.InfraImage = p.InfraImage
	s.SharedNamespaces = p.Share

	// Networking config
	s.NetNS = p.Net.Network
	s.StaticIP = p.Net.StaticIP
	s.StaticMAC = p.Net.StaticMAC
	s.PortMappings = p.Net.PublishPorts
	s.CNINetworks = p.Net.CNINetworks
	if p.Net.UseImageResolvConf {
		s.NoManageResolvConf = true
	}
	s.DNSServer = p.Net.DNSServers
	s.DNSSearch = p.Net.DNSSearch
	s.DNSOption = p.Net.DNSOptions
	s.NoManageHosts = p.Net.NoHosts
	s.HostAdd = p.Net.AddHosts

	// Cgroup
	s.CgroupParent = p.CGroupParent
}

type PodPruneOptions struct {
	Force bool `json:"force" schema:"force"`
}

type PodPruneReport struct {
	Err error
	Id  string
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
	CPU      string
	MemUsage string
	Mem      string
	NetIO    string
	BlockIO  string
	PIDS     string
	Pod      string
	CID      string
	Name     string
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
