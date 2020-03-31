package entities

import (
	"time"

	"github.com/containers/libpod/libpod"
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
	s.InfraCommand = []string{p.InfraCommand}
	s.InfraImage = p.InfraImage
	s.SharedNamespaces = p.Share

	// Networking config
	s.NetNS = p.Net.Network
	s.StaticIP = p.Net.StaticIP
	s.StaticMAC = p.Net.StaticMAC
	s.PortMappings = p.Net.PublishPorts
	s.CNINetworks = p.Net.CNINetworks
	if p.Net.DNSHost {
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
}

type PodInspectReport struct {
	*libpod.PodInspect
}
