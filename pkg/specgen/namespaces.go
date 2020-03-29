package specgen

import (
	"os"

	"github.com/containers/common/pkg/capabilities"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/image"
	"github.com/cri-o/ocicni/pkg/ocicni"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type NamespaceMode string

const (
	// Default indicates the spec generator should determine
	// a sane default
	Default NamespaceMode = "default"
	// Host means the the namespace is derived from
	// the host
	Host NamespaceMode = "host"
	// Path is the path to a namespace
	Path NamespaceMode = "path"
	// FromContainer means namespace is derived from a
	// different container
	FromContainer NamespaceMode = "container"
	// FromPod indicates the namespace is derived from a pod
	FromPod NamespaceMode = "pod"
	// Private indicates the namespace is private
	Private NamespaceMode = "private"
	// NoNetwork indicates no network namespace should
	// be joined.  loopback should still exists
	NoNetwork NamespaceMode = "none"
	// Bridge indicates that a CNI network stack
	// should be used
	Bridge NamespaceMode = "bridge"
	// Slirp indicates that a slirp4ns network stack should
	// be used
	Slirp NamespaceMode = "slirp4ns"
)

// Namespace describes the namespace
type Namespace struct {
	NSMode NamespaceMode `json:"nsmode,omitempty"`
	Value  string        `json:"string,omitempty"`
}

// IsHost returns a bool if the namespace is host based
func (n *Namespace) IsHost() bool {
	return n.NSMode == Host
}

// IsPath indicates via bool if the namespace is based on a path
func (n *Namespace) IsPath() bool {
	return n.NSMode == Path
}

// IsContainer indicates via bool if the namespace is based on a container
func (n *Namespace) IsContainer() bool {
	return n.NSMode == FromContainer
}

// IsPod indicates via bool if the namespace is based on a pod
func (n *Namespace) IsPod() bool {
	return n.NSMode == FromPod
}

// IsPrivate indicates the namespace is private
func (n *Namespace) IsPrivate() bool {
	return n.NSMode == Private
}

func validateNetNS(n *Namespace) error {
	if n == nil {
		return nil
	}
	switch n.NSMode {
	case Host, Path, FromContainer, FromPod, Private, NoNetwork, Bridge, Slirp:
		break
	default:
		return errors.Errorf("invalid network %q", n.NSMode)
	}
	return nil
}

// Validate perform simple validation on the namespace to make sure it is not
// invalid from the get-go
func (n *Namespace) validate() error {
	if n == nil {
		return nil
	}
	// Path and From Container MUST have a string value set
	if n.NSMode == Path || n.NSMode == FromContainer {
		if len(n.Value) < 1 {
			return errors.Errorf("namespace mode %s requires a value", n.NSMode)
		}
	} else {
		// All others must NOT set a string value
		if len(n.Value) > 0 {
			return errors.Errorf("namespace value %s cannot be provided with namespace mode %s", n.Value, n.NSMode)
		}
	}
	return nil
}

func (s *SpecGenerator) GenerateNamespaceContainerOpts(rt *libpod.Runtime) ([]libpod.CtrCreateOption, error) {
	var portBindings []ocicni.PortMapping
	options := make([]libpod.CtrCreateOption, 0)

	// Cgroups
	switch {
	case s.CgroupNS.IsPrivate():
		ns := s.CgroupNS.Value
		if _, err := os.Stat(ns); err != nil {
			return nil, err
		}
	case s.CgroupNS.IsContainer():
		connectedCtr, err := rt.LookupContainer(s.CgroupNS.Value)
		if err != nil {
			return nil, errors.Wrapf(err, "container %q not found", s.CgroupNS.Value)
		}
		options = append(options, libpod.WithCgroupNSFrom(connectedCtr))
		//	TODO
		//default:
		//	return nil, errors.New("cgroup name only supports private and container")
	}

	if s.CgroupParent != "" {
		options = append(options, libpod.WithCgroupParent(s.CgroupParent))
	}

	if s.CgroupsMode != "" {
		options = append(options, libpod.WithCgroupsMode(s.CgroupsMode))
	}

	// ipc
	switch {
	case s.IpcNS.IsHost():
		options = append(options, libpod.WithShmDir("/dev/shm"))
	case s.IpcNS.IsContainer():
		connectedCtr, err := rt.LookupContainer(s.IpcNS.Value)
		if err != nil {
			return nil, errors.Wrapf(err, "container %q not found", s.IpcNS.Value)
		}
		options = append(options, libpod.WithIPCNSFrom(connectedCtr))
		options = append(options, libpod.WithShmDir(connectedCtr.ShmDir()))
	}

	// pid
	if s.PidNS.IsContainer() {
		connectedCtr, err := rt.LookupContainer(s.PidNS.Value)
		if err != nil {
			return nil, errors.Wrapf(err, "container %q not found", s.PidNS.Value)
		}
		options = append(options, libpod.WithPIDNSFrom(connectedCtr))
	}

	// uts
	switch {
	case s.UtsNS.IsPod():
		connectedPod, err := rt.LookupPod(s.UtsNS.Value)
		if err != nil {
			return nil, errors.Wrapf(err, "pod %q not found", s.UtsNS.Value)
		}
		options = append(options, libpod.WithUTSNSFromPod(connectedPod))
	case s.UtsNS.IsContainer():
		connectedCtr, err := rt.LookupContainer(s.UtsNS.Value)
		if err != nil {
			return nil, errors.Wrapf(err, "container %q not found", s.UtsNS.Value)
		}

		options = append(options, libpod.WithUTSNSFrom(connectedCtr))
	}

	if s.UseImageHosts {
		options = append(options, libpod.WithUseImageHosts())
	} else if len(s.HostAdd) > 0 {
		options = append(options, libpod.WithHosts(s.HostAdd))
	}

	// User

	switch {
	case s.UserNS.IsPath():
		ns := s.UserNS.Value
		if ns == "" {
			return nil, errors.Errorf("invalid empty user-defined user namespace")
		}
		_, err := os.Stat(ns)
		if err != nil {
			return nil, err
		}
		if s.IDMappings != nil {
			options = append(options, libpod.WithIDMappings(*s.IDMappings))
		}
	case s.UserNS.IsContainer():
		connectedCtr, err := rt.LookupContainer(s.UserNS.Value)
		if err != nil {
			return nil, errors.Wrapf(err, "container %q not found", s.UserNS.Value)
		}
		options = append(options, libpod.WithUserNSFrom(connectedCtr))
	default:
		if s.IDMappings != nil {
			options = append(options, libpod.WithIDMappings(*s.IDMappings))
		}
	}

	options = append(options, libpod.WithUser(s.User))
	options = append(options, libpod.WithGroups(s.Groups))

	if len(s.PortMappings) > 0 {
		portBindings = s.PortMappings
	}

	switch {
	case s.NetNS.IsPath():
		ns := s.NetNS.Value
		if ns == "" {
			return nil, errors.Errorf("invalid empty user-defined network namespace")
		}
		_, err := os.Stat(ns)
		if err != nil {
			return nil, err
		}
	case s.NetNS.IsContainer():
		connectedCtr, err := rt.LookupContainer(s.NetNS.Value)
		if err != nil {
			return nil, errors.Wrapf(err, "container %q not found", s.NetNS.Value)
		}
		options = append(options, libpod.WithNetNSFrom(connectedCtr))
	case !s.NetNS.IsHost() && s.NetNS.NSMode != NoNetwork:
		postConfigureNetNS := !s.UserNS.IsHost()
		options = append(options, libpod.WithNetNS(portBindings, postConfigureNetNS, string(s.NetNS.NSMode), s.CNINetworks))
	}

	if len(s.DNSSearch) > 0 {
		options = append(options, libpod.WithDNSSearch(s.DNSSearch))
	}
	if len(s.DNSServer) > 0 {
		// TODO I'm not sure how we are going to handle this given the input
		if len(s.DNSServer) == 1 { //&& strings.ToLower(s.DNSServer[0].) == "none" {
			options = append(options, libpod.WithUseImageResolvConf())
		} else {
			var dnsServers []string
			for _, d := range s.DNSServer {
				dnsServers = append(dnsServers, d.String())
			}
			options = append(options, libpod.WithDNS(dnsServers))
		}
	}
	if len(s.DNSOption) > 0 {
		options = append(options, libpod.WithDNSOption(s.DNSOption))
	}
	if s.StaticIP != nil {
		options = append(options, libpod.WithStaticIP(*s.StaticIP))
	}

	if s.StaticMAC != nil {
		options = append(options, libpod.WithStaticMAC(*s.StaticMAC))
	}
	return options, nil
}

func (s *SpecGenerator) pidConfigureGenerator(g *generate.Generator) error {
	if s.PidNS.IsPath() {
		return g.AddOrReplaceLinuxNamespace(string(spec.PIDNamespace), s.PidNS.Value)
	}
	if s.PidNS.IsHost() {
		return g.RemoveLinuxNamespace(string(spec.PIDNamespace))
	}
	if s.PidNS.IsContainer() {
		logrus.Debugf("using container %s pidmode", s.PidNS.Value)
	}
	if s.PidNS.IsPod() {
		logrus.Debug("using pod pidmode")
	}
	return nil
}

func (s *SpecGenerator) utsConfigureGenerator(g *generate.Generator, runtime *libpod.Runtime) error {
	hostname := s.Hostname
	var err error
	if hostname == "" {
		switch {
		case s.UtsNS.IsContainer():
			utsCtr, err := runtime.LookupContainer(s.UtsNS.Value)
			if err != nil {
				return errors.Wrapf(err, "unable to retrieve hostname from dependency container %s", s.UtsNS.Value)
			}
			hostname = utsCtr.Hostname()
		case s.NetNS.IsHost() || s.UtsNS.IsHost():
			hostname, err = os.Hostname()
			if err != nil {
				return errors.Wrap(err, "unable to retrieve hostname of the host")
			}
		default:
			logrus.Debug("No hostname set; container's hostname will default to runtime default")
		}
	}
	g.RemoveHostname()
	if s.Hostname != "" || !s.UtsNS.IsHost() {
		// Set the hostname in the OCI configuration only
		// if specified by the user or if we are creating
		// a new UTS namespace.
		g.SetHostname(hostname)
	}
	g.AddProcessEnv("HOSTNAME", hostname)

	if s.UtsNS.IsPath() {
		return g.AddOrReplaceLinuxNamespace(string(spec.UTSNamespace), s.UtsNS.Value)
	}
	if s.UtsNS.IsHost() {
		return g.RemoveLinuxNamespace(string(spec.UTSNamespace))
	}
	if s.UtsNS.IsContainer() {
		logrus.Debugf("using container %s utsmode", s.UtsNS.Value)
	}
	return nil
}

func (s *SpecGenerator) ipcConfigureGenerator(g *generate.Generator) error {
	if s.IpcNS.IsPath() {
		return g.AddOrReplaceLinuxNamespace(string(spec.IPCNamespace), s.IpcNS.Value)
	}
	if s.IpcNS.IsHost() {
		return g.RemoveLinuxNamespace(s.IpcNS.Value)
	}
	if s.IpcNS.IsContainer() {
		logrus.Debugf("Using container %s ipcmode", s.IpcNS.Value)
	}
	return nil
}

func (s *SpecGenerator) cgroupConfigureGenerator(g *generate.Generator) error {
	if s.CgroupNS.IsPath() {
		return g.AddOrReplaceLinuxNamespace(string(spec.CgroupNamespace), s.CgroupNS.Value)
	}
	if s.CgroupNS.IsHost() {
		return g.RemoveLinuxNamespace(s.CgroupNS.Value)
	}
	if s.CgroupNS.IsPrivate() {
		return g.AddOrReplaceLinuxNamespace(string(spec.CgroupNamespace), "")
	}
	if s.CgroupNS.IsContainer() {
		logrus.Debugf("Using container %s cgroup mode", s.CgroupNS.Value)
	}
	return nil
}

func (s *SpecGenerator) networkConfigureGenerator(g *generate.Generator) error {
	switch {
	case s.NetNS.IsHost():
		logrus.Debug("Using host netmode")
		if err := g.RemoveLinuxNamespace(string(spec.NetworkNamespace)); err != nil {
			return err
		}

	case s.NetNS.NSMode == NoNetwork:
		logrus.Debug("Using none netmode")
	case s.NetNS.NSMode == Bridge:
		logrus.Debug("Using bridge netmode")
	case s.NetNS.IsContainer():
		logrus.Debugf("using container %s netmode", s.NetNS.Value)
	case s.NetNS.IsPath():
		logrus.Debug("Using ns netmode")
		if err := g.AddOrReplaceLinuxNamespace(string(spec.NetworkNamespace), s.NetNS.Value); err != nil {
			return err
		}
	case s.NetNS.IsPod():
		logrus.Debug("Using pod netmode, unless pod is not sharing")
	case s.NetNS.NSMode == Slirp:
		logrus.Debug("Using slirp4netns netmode")
	default:
		return errors.Errorf("unknown network mode")
	}

	if g.Config.Annotations == nil {
		g.Config.Annotations = make(map[string]string)
	}

	if s.PublishImagePorts {
		g.Config.Annotations[libpod.InspectAnnotationPublishAll] = libpod.InspectResponseTrue
	} else {
		g.Config.Annotations[libpod.InspectAnnotationPublishAll] = libpod.InspectResponseFalse
	}

	return nil
}

func (s *SpecGenerator) userConfigureGenerator(g *generate.Generator) error {
	if s.UserNS.IsPath() {
		if err := g.AddOrReplaceLinuxNamespace(string(spec.UserNamespace), s.UserNS.Value); err != nil {
			return err
		}
		// runc complains if no mapping is specified, even if we join another ns.  So provide a dummy mapping
		g.AddLinuxUIDMapping(uint32(0), uint32(0), uint32(1))
		g.AddLinuxGIDMapping(uint32(0), uint32(0), uint32(1))
	}

	if s.IDMappings != nil {
		if (len(s.IDMappings.UIDMap) > 0 || len(s.IDMappings.GIDMap) > 0) && !s.UserNS.IsHost() {
			if err := g.AddOrReplaceLinuxNamespace(string(spec.UserNamespace), ""); err != nil {
				return err
			}
		}
		for _, uidmap := range s.IDMappings.UIDMap {
			g.AddLinuxUIDMapping(uint32(uidmap.HostID), uint32(uidmap.ContainerID), uint32(uidmap.Size))
		}
		for _, gidmap := range s.IDMappings.GIDMap {
			g.AddLinuxGIDMapping(uint32(gidmap.HostID), uint32(gidmap.ContainerID), uint32(gidmap.Size))
		}
	}
	return nil
}

func (s *SpecGenerator) securityConfigureGenerator(g *generate.Generator, newImage *image.Image) error {
	// HANDLE CAPABILITIES
	// NOTE: Must happen before SECCOMP
	if s.Privileged {
		g.SetupPrivileged(true)
	}

	useNotRoot := func(user string) bool {
		if user == "" || user == "root" || user == "0" {
			return false
		}
		return true
	}
	configSpec := g.Config
	var err error
	var caplist []string
	bounding := configSpec.Process.Capabilities.Bounding
	if useNotRoot(s.User) {
		configSpec.Process.Capabilities.Bounding = caplist
	}
	caplist, err = capabilities.MergeCapabilities(configSpec.Process.Capabilities.Bounding, s.CapAdd, s.CapDrop)
	if err != nil {
		return err
	}

	configSpec.Process.Capabilities.Bounding = caplist
	configSpec.Process.Capabilities.Permitted = caplist
	configSpec.Process.Capabilities.Inheritable = caplist
	configSpec.Process.Capabilities.Effective = caplist
	configSpec.Process.Capabilities.Ambient = caplist
	if useNotRoot(s.User) {
		caplist, err = capabilities.MergeCapabilities(bounding, s.CapAdd, s.CapDrop)
		if err != nil {
			return err
		}
	}
	configSpec.Process.Capabilities.Bounding = caplist

	// HANDLE SECCOMP
	if s.SeccompProfilePath != "unconfined" {
		seccompConfig, err := s.getSeccompConfig(configSpec, newImage)
		if err != nil {
			return err
		}
		configSpec.Linux.Seccomp = seccompConfig
	}

	// Clear default Seccomp profile from Generator for privileged containers
	if s.SeccompProfilePath == "unconfined" || s.Privileged {
		configSpec.Linux.Seccomp = nil
	}

	g.SetRootReadonly(s.ReadOnlyFilesystem)
	for sysctlKey, sysctlVal := range s.Sysctl {
		g.AddLinuxSysctl(sysctlKey, sysctlVal)
	}

	return nil
}
