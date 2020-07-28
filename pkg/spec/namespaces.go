package createconfig

import (
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/containers/podman/v2/libpod"
	"github.com/containers/podman/v2/libpod/define"
	"github.com/containers/podman/v2/pkg/cgroups"
	"github.com/cri-o/ocicni/pkg/ocicni"
	"github.com/docker/go-connections/nat"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// DefaultKernelNamespaces is a comma-separated list of default kernel
// namespaces.
const DefaultKernelNamespaces = "cgroup,ipc,net,uts"

// ToCreateOptions converts the input to a slice of container create options.
func (c *NetworkConfig) ToCreateOptions(runtime *libpod.Runtime, userns *UserConfig) ([]libpod.CtrCreateOption, error) {
	var portBindings []ocicni.PortMapping
	var err error
	if len(c.PortBindings) > 0 {
		portBindings, err = NatToOCIPortBindings(c.PortBindings)
		if err != nil {
			return nil, errors.Wrapf(err, "unable to create port bindings")
		}
	}

	options := make([]libpod.CtrCreateOption, 0)
	userNetworks := c.NetMode.UserDefined()
	networks := make([]string, 0)

	if IsPod(userNetworks) {
		userNetworks = ""
	}
	if userNetworks != "" {
		for _, netName := range strings.Split(userNetworks, ",") {
			if netName == "" {
				return nil, errors.Errorf("container networks %q invalid", userNetworks)
			}
			networks = append(networks, netName)
		}
	}

	switch {
	case c.NetMode.IsNS():
		ns := c.NetMode.NS()
		if ns == "" {
			return nil, errors.Errorf("invalid empty user-defined network namespace")
		}
		_, err := os.Stat(ns)
		if err != nil {
			return nil, err
		}
	case c.NetMode.IsContainer():
		connectedCtr, err := runtime.LookupContainer(c.NetMode.Container())
		if err != nil {
			return nil, errors.Wrapf(err, "container %q not found", c.NetMode.Container())
		}
		options = append(options, libpod.WithNetNSFrom(connectedCtr))
	case !c.NetMode.IsHost() && !c.NetMode.IsNone():
		postConfigureNetNS := userns.getPostConfigureNetNS()
		options = append(options, libpod.WithNetNS(portBindings, postConfigureNetNS, string(c.NetMode), networks))
	}

	if len(c.DNSSearch) > 0 {
		options = append(options, libpod.WithDNSSearch(c.DNSSearch))
	}
	if len(c.DNSServers) > 0 {
		if len(c.DNSServers) == 1 && strings.ToLower(c.DNSServers[0]) == "none" {
			options = append(options, libpod.WithUseImageResolvConf())
		} else {
			options = append(options, libpod.WithDNS(c.DNSServers))
		}
	}
	if len(c.DNSOpt) > 0 {
		options = append(options, libpod.WithDNSOption(c.DNSOpt))
	}
	if c.IPAddress != "" {
		ip := net.ParseIP(c.IPAddress)
		if ip == nil {
			return nil, errors.Wrapf(define.ErrInvalidArg, "cannot parse %s as IP address", c.IPAddress)
		} else if ip.To4() == nil {
			return nil, errors.Wrapf(define.ErrInvalidArg, "%s is not an IPv4 address", c.IPAddress)
		}
		options = append(options, libpod.WithStaticIP(ip))
	}

	if c.MacAddress != "" {
		mac, err := net.ParseMAC(c.MacAddress)
		if err != nil {
			return nil, errors.Wrapf(define.ErrInvalidArg, "cannot parse %s as MAC address: %v", c.MacAddress, err)
		}
		options = append(options, libpod.WithStaticMAC(mac))
	}

	return options, nil
}

// ConfigureGenerator configures the generator based according to the current
// state of the NetworkConfig.
func (c *NetworkConfig) ConfigureGenerator(g *generate.Generator) error {
	netMode := c.NetMode
	netCtr := netMode.Container()
	switch {
	case netMode.IsHost():
		logrus.Debug("Using host netmode")
		if err := g.RemoveLinuxNamespace(string(spec.NetworkNamespace)); err != nil {
			return err
		}
	case netMode.IsNone():
		logrus.Debug("Using none netmode")
	case netMode.IsBridge():
		logrus.Debug("Using bridge netmode")
	case netCtr != "":
		logrus.Debugf("using container %s netmode", netCtr)
	case IsNS(string(netMode)):
		logrus.Debug("Using ns netmode")
		if err := g.AddOrReplaceLinuxNamespace(string(spec.NetworkNamespace), NS(string(netMode))); err != nil {
			return err
		}
	case IsPod(string(netMode)):
		logrus.Debug("Using pod netmode, unless pod is not sharing")
	case netMode.IsSlirp4netns():
		logrus.Debug("Using slirp4netns netmode")
	case netMode.IsUserDefined():
		logrus.Debug("Using user defined netmode")
	default:
		return errors.Errorf("unknown network mode")
	}

	if c.HTTPProxy {
		for _, envSpec := range []string{
			"http_proxy",
			"HTTP_PROXY",
			"https_proxy",
			"HTTPS_PROXY",
			"ftp_proxy",
			"FTP_PROXY",
			"no_proxy",
			"NO_PROXY",
		} {
			envVal := os.Getenv(envSpec)
			if envVal != "" {
				g.AddProcessEnv(envSpec, envVal)
			}
		}
	}

	if g.Config.Annotations == nil {
		g.Config.Annotations = make(map[string]string)
	}

	if c.PublishAll {
		g.Config.Annotations[define.InspectAnnotationPublishAll] = define.InspectResponseTrue
	} else {
		g.Config.Annotations[define.InspectAnnotationPublishAll] = define.InspectResponseFalse
	}

	return nil
}

// NatToOCIPortBindings iterates a nat.portmap slice and creates []ocicni portmapping slice
func NatToOCIPortBindings(ports nat.PortMap) ([]ocicni.PortMapping, error) {
	var portBindings []ocicni.PortMapping
	for containerPb, hostPb := range ports {
		var pm ocicni.PortMapping
		pm.ContainerPort = int32(containerPb.Int())
		for _, i := range hostPb {
			var hostPort int
			var err error
			pm.HostIP = i.HostIP
			if i.HostPort == "" {
				hostPort = containerPb.Int()
			} else {
				hostPort, err = strconv.Atoi(i.HostPort)
				if err != nil {
					return nil, errors.Wrapf(err, "unable to convert host port to integer")
				}
			}

			pm.HostPort = int32(hostPort)
			pm.Protocol = containerPb.Proto()
			portBindings = append(portBindings, pm)
		}
	}
	return portBindings, nil
}

// ToCreateOptions converts the input to container create options.
func (c *CgroupConfig) ToCreateOptions(runtime *libpod.Runtime) ([]libpod.CtrCreateOption, error) {
	options := make([]libpod.CtrCreateOption, 0)
	if c.CgroupMode.IsNS() {
		ns := c.CgroupMode.NS()
		if ns == "" {
			return nil, errors.Errorf("invalid empty user-defined network namespace")
		}
		_, err := os.Stat(ns)
		if err != nil {
			return nil, err
		}
	} else if c.CgroupMode.IsContainer() {
		connectedCtr, err := runtime.LookupContainer(c.CgroupMode.Container())
		if err != nil {
			return nil, errors.Wrapf(err, "container %q not found", c.CgroupMode.Container())
		}
		options = append(options, libpod.WithCgroupNSFrom(connectedCtr))
	}

	if c.CgroupParent != "" {
		options = append(options, libpod.WithCgroupParent(c.CgroupParent))
	}

	if c.Cgroups != "" {
		options = append(options, libpod.WithCgroupsMode(c.Cgroups))
	}

	return options, nil
}

// ToCreateOptions converts the input to container create options.
func (c *UserConfig) ToCreateOptions(runtime *libpod.Runtime) ([]libpod.CtrCreateOption, error) {
	options := make([]libpod.CtrCreateOption, 0)
	switch {
	case c.UsernsMode.IsNS():
		ns := c.UsernsMode.NS()
		if ns == "" {
			return nil, errors.Errorf("invalid empty user-defined user namespace")
		}
		_, err := os.Stat(ns)
		if err != nil {
			return nil, err
		}
		options = append(options, libpod.WithIDMappings(*c.IDMappings))
	case c.UsernsMode.IsContainer():
		connectedCtr, err := runtime.LookupContainer(c.UsernsMode.Container())
		if err != nil {
			return nil, errors.Wrapf(err, "container %q not found", c.UsernsMode.Container())
		}
		options = append(options, libpod.WithUserNSFrom(connectedCtr))
	default:
		options = append(options, libpod.WithIDMappings(*c.IDMappings))
	}

	options = append(options, libpod.WithUser(c.User))
	options = append(options, libpod.WithGroups(c.GroupAdd))

	return options, nil
}

// ConfigureGenerator configures the generator according to the current state
// of the UserConfig.
func (c *UserConfig) ConfigureGenerator(g *generate.Generator) error {
	if IsNS(string(c.UsernsMode)) {
		if err := g.AddOrReplaceLinuxNamespace(string(spec.UserNamespace), NS(string(c.UsernsMode))); err != nil {
			return err
		}
		// runc complains if no mapping is specified, even if we join another ns.  So provide a dummy mapping
		g.AddLinuxUIDMapping(uint32(0), uint32(0), uint32(1))
		g.AddLinuxGIDMapping(uint32(0), uint32(0), uint32(1))
	}

	if (len(c.IDMappings.UIDMap) > 0 || len(c.IDMappings.GIDMap) > 0) && !c.UsernsMode.IsHost() {
		if err := g.AddOrReplaceLinuxNamespace(string(spec.UserNamespace), ""); err != nil {
			return err
		}
	}
	for _, uidmap := range c.IDMappings.UIDMap {
		g.AddLinuxUIDMapping(uint32(uidmap.HostID), uint32(uidmap.ContainerID), uint32(uidmap.Size))
	}
	for _, gidmap := range c.IDMappings.GIDMap {
		g.AddLinuxGIDMapping(uint32(gidmap.HostID), uint32(gidmap.ContainerID), uint32(gidmap.Size))
	}
	return nil
}

func (c *UserConfig) getPostConfigureNetNS() bool {
	hasUserns := c.UsernsMode.IsContainer() || c.UsernsMode.IsNS() || c.UsernsMode.IsAuto() || len(c.IDMappings.UIDMap) > 0 || len(c.IDMappings.GIDMap) > 0
	postConfigureNetNS := hasUserns && !c.UsernsMode.IsHost()
	return postConfigureNetNS
}

// InNS returns true if the UserConfig indicates to be in a dedicated user
// namespace.
func (c *UserConfig) InNS(isRootless bool) bool {
	hasUserns := c.UsernsMode.IsContainer() || c.UsernsMode.IsNS() || c.UsernsMode.IsAuto() || len(c.IDMappings.UIDMap) > 0 || len(c.IDMappings.GIDMap) > 0
	return isRootless || (hasUserns && !c.UsernsMode.IsHost())
}

// ToCreateOptions converts the input to container create options.
func (c *IpcConfig) ToCreateOptions(runtime *libpod.Runtime) ([]libpod.CtrCreateOption, error) {
	options := make([]libpod.CtrCreateOption, 0)
	if c.IpcMode.IsHost() {
		options = append(options, libpod.WithShmDir("/dev/shm"))
	} else if c.IpcMode.IsContainer() {
		connectedCtr, err := runtime.LookupContainer(c.IpcMode.Container())
		if err != nil {
			return nil, errors.Wrapf(err, "container %q not found", c.IpcMode.Container())
		}

		options = append(options, libpod.WithIPCNSFrom(connectedCtr))
		options = append(options, libpod.WithShmDir(connectedCtr.ShmDir()))
	}

	return options, nil
}

// ConfigureGenerator configures the generator according to the current state
// of the IpcConfig.
func (c *IpcConfig) ConfigureGenerator(g *generate.Generator) error {
	ipcMode := c.IpcMode
	if IsNS(string(ipcMode)) {
		return g.AddOrReplaceLinuxNamespace(string(spec.IPCNamespace), NS(string(ipcMode)))
	}
	if ipcMode.IsHost() {
		return g.RemoveLinuxNamespace(string(spec.IPCNamespace))
	}
	if ipcCtr := ipcMode.Container(); ipcCtr != "" {
		logrus.Debugf("Using container %s ipcmode", ipcCtr)
	}

	return nil
}

// ConfigureGenerator configures the generator according to the current state
// of the CgroupConfig.
func (c *CgroupConfig) ConfigureGenerator(g *generate.Generator) error {
	cgroupMode := c.CgroupMode
	if cgroupMode.IsDefaultValue() {
		// If the value is not specified, default to "private" on cgroups v2 and "host" on cgroups v1.
		unified, err := cgroups.IsCgroup2UnifiedMode()
		if err != nil {
			return err
		}
		if unified {
			cgroupMode = "private"
		} else {
			cgroupMode = "host"
		}
	}
	if cgroupMode.IsNS() {
		return g.AddOrReplaceLinuxNamespace(string(spec.CgroupNamespace), NS(string(cgroupMode)))
	}
	if cgroupMode.IsHost() {
		return g.RemoveLinuxNamespace(string(spec.CgroupNamespace))
	}
	if cgroupMode.IsPrivate() {
		return g.AddOrReplaceLinuxNamespace(string(spec.CgroupNamespace), "")
	}
	if cgCtr := cgroupMode.Container(); cgCtr != "" {
		logrus.Debugf("Using container %s cgroup mode", cgCtr)
	}
	return nil
}

// ToCreateOptions converts the input to container create options.
func (c *PidConfig) ToCreateOptions(runtime *libpod.Runtime) ([]libpod.CtrCreateOption, error) {
	options := make([]libpod.CtrCreateOption, 0)
	if c.PidMode.IsContainer() {
		connectedCtr, err := runtime.LookupContainer(c.PidMode.Container())
		if err != nil {
			return nil, errors.Wrapf(err, "container %q not found", c.PidMode.Container())
		}

		options = append(options, libpod.WithPIDNSFrom(connectedCtr))
	}

	return options, nil
}

// ConfigureGenerator configures the generator according to the current state
// of the PidConfig.
func (c *PidConfig) ConfigureGenerator(g *generate.Generator) error {
	pidMode := c.PidMode
	if IsNS(string(pidMode)) {
		return g.AddOrReplaceLinuxNamespace(string(spec.PIDNamespace), NS(string(pidMode)))
	}
	if pidMode.IsHost() {
		return g.RemoveLinuxNamespace(string(spec.PIDNamespace))
	}
	if pidCtr := pidMode.Container(); pidCtr != "" {
		logrus.Debugf("using container %s pidmode", pidCtr)
	}
	if IsPod(string(pidMode)) {
		logrus.Debug("using pod pidmode")
	}
	return nil
}

// ToCreateOptions converts the input to container create options.
func (c *UtsConfig) ToCreateOptions(runtime *libpod.Runtime, pod *libpod.Pod) ([]libpod.CtrCreateOption, error) {
	options := make([]libpod.CtrCreateOption, 0)
	if IsPod(string(c.UtsMode)) {
		options = append(options, libpod.WithUTSNSFromPod(pod))
	}
	if c.UtsMode.IsContainer() {
		connectedCtr, err := runtime.LookupContainer(c.UtsMode.Container())
		if err != nil {
			return nil, errors.Wrapf(err, "container %q not found", c.UtsMode.Container())
		}

		options = append(options, libpod.WithUTSNSFrom(connectedCtr))
	}
	if c.NoHosts {
		options = append(options, libpod.WithUseImageHosts())
	}
	if len(c.HostAdd) > 0 && !c.NoHosts {
		options = append(options, libpod.WithHosts(c.HostAdd))
	}

	return options, nil
}

// ConfigureGenerator configures the generator according to the current state
// of the UtsConfig.
func (c *UtsConfig) ConfigureGenerator(g *generate.Generator, net *NetworkConfig, runtime *libpod.Runtime) error {
	hostname := c.Hostname
	utsCtrID := c.UtsMode.Container()
	var err error
	if hostname == "" {
		switch {
		case utsCtrID != "":
			utsCtr, err := runtime.LookupContainer(utsCtrID)
			if err != nil {
				return errors.Wrapf(err, "unable to retrieve hostname from dependency container %s", utsCtrID)
			}
			hostname = utsCtr.Hostname()
		case net.NetMode.IsHost() || c.UtsMode.IsHost():
			hostname, err = os.Hostname()
			if err != nil {
				return errors.Wrap(err, "unable to retrieve hostname of the host")
			}
		default:
			logrus.Debug("No hostname set; container's hostname will default to runtime default")
		}
	}
	g.RemoveHostname()
	if c.Hostname != "" || !c.UtsMode.IsHost() {
		// Set the hostname in the OCI configuration only
		// if specified by the user or if we are creating
		// a new UTS namespace.
		g.SetHostname(hostname)
	}
	g.AddProcessEnv("HOSTNAME", hostname)

	utsMode := c.UtsMode
	if IsNS(string(utsMode)) {
		return g.AddOrReplaceLinuxNamespace(string(spec.UTSNamespace), NS(string(utsMode)))
	}
	if utsMode.IsHost() {
		return g.RemoveLinuxNamespace(string(spec.UTSNamespace))
	}
	if utsCtr := utsMode.Container(); utsCtr != "" {
		logrus.Debugf("using container %s utsmode", utsCtr)
	}
	return nil
}
