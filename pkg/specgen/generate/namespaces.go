package generate

import (
	"fmt"
	"os"
	"strings"

	"github.com/containers/common/libimage"
	"github.com/containers/common/libnetwork/types"
	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v4/libpod"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/rootless"
	"github.com/containers/podman/v4/pkg/specgen"
	"github.com/containers/podman/v4/pkg/util"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// Get the default namespace mode for any given namespace type.
func GetDefaultNamespaceMode(nsType string, cfg *config.Config, pod *libpod.Pod) (specgen.Namespace, error) {
	// The default for most is private
	toReturn := specgen.Namespace{}
	toReturn.NSMode = specgen.Private

	// Ensure case insensitivity
	nsType = strings.ToLower(nsType)

	// If the pod is not nil - check shared namespaces
	if pod != nil && pod.HasInfraContainer() {
		podMode := false
		switch {
		case nsType == "pid" && pod.SharesPID():
			podMode = true
		case nsType == "ipc" && pod.SharesIPC():
			podMode = true
		case nsType == "uts" && pod.SharesUTS():
			podMode = true
		case nsType == "user" && pod.SharesUser():
			podMode = true
		case nsType == "net" && pod.SharesNet():
			podMode = true
		case nsType == "cgroup" && pod.SharesCgroup():
			podMode = true
		}
		if podMode {
			toReturn.NSMode = specgen.FromPod
			return toReturn, nil
		}
	}

	if cfg == nil {
		cfg = &config.Config{}
	}
	switch nsType {
	case "pid":
		return specgen.ParseNamespace(cfg.Containers.PidNS)
	case "ipc":
		return specgen.ParseIPCNamespace(cfg.Containers.IPCNS)
	case "uts":
		return specgen.ParseNamespace(cfg.Containers.UTSNS)
	case "user":
		return specgen.ParseUserNamespace(cfg.Containers.UserNS)
	case "cgroup":
		return specgen.ParseCgroupNamespace(cfg.Containers.CgroupNS)
	case "net":
		ns, _, _, err := specgen.ParseNetworkFlag(nil)
		return ns, err
	}

	return toReturn, errors.Wrapf(define.ErrInvalidArg, "invalid namespace type %q passed", nsType)
}

// namespaceOptions generates container creation options for all
// namespaces in a SpecGenerator.
// Pod is the pod the container will join. May be nil is the container is not
// joining a pod.
// TODO: Consider grouping options that are not directly attached to a namespace
// elsewhere.
func namespaceOptions(s *specgen.SpecGenerator, rt *libpod.Runtime, pod *libpod.Pod, imageData *libimage.ImageData) ([]libpod.CtrCreateOption, error) {
	toReturn := []libpod.CtrCreateOption{}

	// If pod is not nil, get infra container.
	var infraCtr *libpod.Container
	if pod != nil {
		infraID, err := pod.InfraContainerID()
		if err != nil {
			// This is likely to be of the fatal kind (pod was
			// removed) so hard fail
			return nil, errors.Wrapf(err, "error looking up pod %s infra container", pod.ID())
		}
		if infraID != "" {
			ctr, err := rt.GetContainer(infraID)
			if err != nil {
				return nil, errors.Wrapf(err, "error retrieving pod %s infra container %s", pod.ID(), infraID)
			}
			infraCtr = ctr
		}
	}

	errNoInfra := errors.Wrapf(define.ErrInvalidArg, "cannot use pod namespace as container is not joining a pod or pod has no infra container")

	// PID
	switch s.PidNS.NSMode {
	case specgen.FromPod:
		if pod == nil || infraCtr == nil {
			return nil, errNoInfra
		}
		toReturn = append(toReturn, libpod.WithPIDNSFrom(infraCtr))
	case specgen.FromContainer:
		pidCtr, err := rt.LookupContainer(s.PidNS.Value)
		if err != nil {
			return nil, errors.Wrapf(err, "error looking up container to share pid namespace with")
		}
		toReturn = append(toReturn, libpod.WithPIDNSFrom(pidCtr))
	}

	// IPC
	switch s.IpcNS.NSMode {
	case specgen.Host:
		// Force use of host /dev/shm for host namespace
		toReturn = append(toReturn, libpod.WithShmDir("/dev/shm"))
	case specgen.FromPod:
		if pod == nil || infraCtr == nil {
			return nil, errNoInfra
		}
		toReturn = append(toReturn, libpod.WithIPCNSFrom(infraCtr))
		toReturn = append(toReturn, libpod.WithShmDir(infraCtr.ShmDir()))
	case specgen.FromContainer:
		ipcCtr, err := rt.LookupContainer(s.IpcNS.Value)
		if err != nil {
			return nil, errors.Wrapf(err, "error looking up container to share ipc namespace with")
		}
		if ipcCtr.ConfigNoCopy().NoShmShare {
			return nil, errors.Errorf("joining IPC of container %s is not allowed: non-shareable IPC (hint: use IpcMode:shareable for the donor container)", ipcCtr.ID())
		}
		toReturn = append(toReturn, libpod.WithIPCNSFrom(ipcCtr))
		if !ipcCtr.ConfigNoCopy().NoShm {
			toReturn = append(toReturn, libpod.WithShmDir(ipcCtr.ShmDir()))
		}
	case specgen.None:
		toReturn = append(toReturn, libpod.WithNoShm(true))
	case specgen.Private:
		toReturn = append(toReturn, libpod.WithNoShmShare(true))
	}

	// UTS
	switch s.UtsNS.NSMode {
	case specgen.FromPod:
		if pod == nil || infraCtr == nil {
			return nil, errNoInfra
		}
		toReturn = append(toReturn, libpod.WithUTSNSFrom(infraCtr))
	case specgen.FromContainer:
		utsCtr, err := rt.LookupContainer(s.UtsNS.Value)
		if err != nil {
			return nil, errors.Wrapf(err, "error looking up container to share uts namespace with")
		}
		toReturn = append(toReturn, libpod.WithUTSNSFrom(utsCtr))
	}

	// User
	switch s.UserNS.NSMode {
	case specgen.KeepID:
		if !rootless.IsRootless() {
			return nil, errors.New("keep-id is only supported in rootless mode")
		}
		toReturn = append(toReturn, libpod.WithAddCurrentUserPasswdEntry())

		// If user is not overridden, set user in the container
		// to user running Podman.
		if s.User == "" {
			_, uid, gid, err := util.GetKeepIDMapping()
			if err != nil {
				return nil, err
			}
			toReturn = append(toReturn, libpod.WithUser(fmt.Sprintf("%d:%d", uid, gid)))
		}
	case specgen.FromPod:
		if pod == nil || infraCtr == nil {
			return nil, errNoInfra
		}
		// Inherit the user from the infra container if it is set and --user has not
		// been set explicitly
		if infraCtr.User() != "" && s.User == "" {
			toReturn = append(toReturn, libpod.WithUser(infraCtr.User()))
		}
		toReturn = append(toReturn, libpod.WithUserNSFrom(infraCtr))
	case specgen.FromContainer:
		userCtr, err := rt.LookupContainer(s.UserNS.Value)
		if err != nil {
			return nil, errors.Wrapf(err, "error looking up container to share user namespace with")
		}
		toReturn = append(toReturn, libpod.WithUserNSFrom(userCtr))
	}

	// This wipes the UserNS settings that get set from the infra container
	// when we are inheritting from the pod. So only apply this if the container
	// is not being created in a pod.
	if s.IDMappings != nil {
		if pod == nil {
			toReturn = append(toReturn, libpod.WithIDMappings(*s.IDMappings))
		} else if pod.HasInfraContainer() && (len(s.IDMappings.UIDMap) > 0 || len(s.IDMappings.GIDMap) > 0) {
			return nil, errors.Wrapf(define.ErrInvalidArg, "cannot specify a new uid/gid map when entering a pod with an infra container")
		}
	}
	if s.User != "" {
		toReturn = append(toReturn, libpod.WithUser(s.User))
	}
	if len(s.Groups) > 0 {
		toReturn = append(toReturn, libpod.WithGroups(s.Groups))
	}

	// Cgroup
	switch s.CgroupNS.NSMode {
	case specgen.FromPod:
		if pod == nil || infraCtr == nil {
			return nil, errNoInfra
		}
		toReturn = append(toReturn, libpod.WithCgroupNSFrom(infraCtr))
	case specgen.FromContainer:
		cgroupCtr, err := rt.LookupContainer(s.CgroupNS.Value)
		if err != nil {
			return nil, errors.Wrapf(err, "error looking up container to share cgroup namespace with")
		}
		toReturn = append(toReturn, libpod.WithCgroupNSFrom(cgroupCtr))
	}

	if s.CgroupParent != "" {
		toReturn = append(toReturn, libpod.WithCgroupParent(s.CgroupParent))
	}

	if s.CgroupsMode != "" {
		toReturn = append(toReturn, libpod.WithCgroupsMode(s.CgroupsMode))
	}

	// Net
	// TODO validate CNINetworks, StaticIP, StaticIPv6 are only set if we
	// are in bridge mode.
	postConfigureNetNS := !s.UserNS.IsHost()
	switch s.NetNS.NSMode {
	case specgen.FromPod:
		if pod == nil || infraCtr == nil {
			return nil, errNoInfra
		}
		toReturn = append(toReturn, libpod.WithNetNSFrom(infraCtr))
	case specgen.FromContainer:
		netCtr, err := rt.LookupContainer(s.NetNS.Value)
		if err != nil {
			return nil, errors.Wrapf(err, "error looking up container to share net namespace with")
		}
		toReturn = append(toReturn, libpod.WithNetNSFrom(netCtr))
	case specgen.Slirp:
		portMappings, expose, err := createPortMappings(s, imageData)
		if err != nil {
			return nil, err
		}
		val := "slirp4netns"
		if s.NetNS.Value != "" {
			val = fmt.Sprintf("slirp4netns:%s", s.NetNS.Value)
		}
		toReturn = append(toReturn, libpod.WithNetNS(portMappings, expose, postConfigureNetNS, val, nil))
	case specgen.Private:
		fallthrough
	case specgen.Bridge:
		portMappings, expose, err := createPortMappings(s, imageData)
		if err != nil {
			return nil, err
		}

		rtConfig, err := rt.GetConfigNoCopy()
		if err != nil {
			return nil, err
		}
		// if no network was specified use add the default
		if len(s.Networks) == 0 {
			// backwards config still allow the old cni networks list and convert to new format
			if len(s.CNINetworks) > 0 {
				logrus.Warn(`specgen "cni_networks" option is deprecated use the "networks" map instead`)
				networks := make(map[string]types.PerNetworkOptions, len(s.CNINetworks))
				for _, net := range s.CNINetworks {
					networks[net] = types.PerNetworkOptions{}
				}
				s.Networks = networks
			} else {
				// no networks given but bridge is set so use default network
				s.Networks = map[string]types.PerNetworkOptions{
					rtConfig.Network.DefaultNetwork: {},
				}
			}
		}
		// rename the "default" network to the correct default name
		if opts, ok := s.Networks["default"]; ok {
			s.Networks[rtConfig.Network.DefaultNetwork] = opts
			delete(s.Networks, "default")
		}
		toReturn = append(toReturn, libpod.WithNetNS(portMappings, expose, postConfigureNetNS, "bridge", s.Networks))
	}

	if s.UseImageHosts {
		toReturn = append(toReturn, libpod.WithUseImageHosts())
	} else if len(s.HostAdd) > 0 {
		toReturn = append(toReturn, libpod.WithHosts(s.HostAdd))
	}
	if len(s.DNSSearch) > 0 {
		toReturn = append(toReturn, libpod.WithDNSSearch(s.DNSSearch))
	}
	if s.UseImageResolvConf {
		toReturn = append(toReturn, libpod.WithUseImageResolvConf())
	} else if len(s.DNSServers) > 0 {
		var dnsServers []string
		for _, d := range s.DNSServers {
			dnsServers = append(dnsServers, d.String())
		}
		toReturn = append(toReturn, libpod.WithDNS(dnsServers))
	}
	if len(s.DNSOptions) > 0 {
		toReturn = append(toReturn, libpod.WithDNSOption(s.DNSOptions))
	}
	if s.NetworkOptions != nil {
		toReturn = append(toReturn, libpod.WithNetworkOptions(s.NetworkOptions))
	}

	return toReturn, nil
}

func specConfigureNamespaces(s *specgen.SpecGenerator, g *generate.Generator, rt *libpod.Runtime, pod *libpod.Pod) error {
	// PID
	switch s.PidNS.NSMode {
	case specgen.Path:
		if _, err := os.Stat(s.PidNS.Value); err != nil {
			return errors.Wrap(err, "cannot find specified PID namespace path")
		}
		if err := g.AddOrReplaceLinuxNamespace(string(spec.PIDNamespace), s.PidNS.Value); err != nil {
			return err
		}
	case specgen.Host:
		if err := g.RemoveLinuxNamespace(string(spec.PIDNamespace)); err != nil {
			return err
		}
	case specgen.Private:
		if err := g.AddOrReplaceLinuxNamespace(string(spec.PIDNamespace), ""); err != nil {
			return err
		}
	}

	// IPC
	switch s.IpcNS.NSMode {
	case specgen.Path:
		if _, err := os.Stat(s.IpcNS.Value); err != nil {
			return errors.Wrap(err, "cannot find specified IPC namespace path")
		}
		if err := g.AddOrReplaceLinuxNamespace(string(spec.IPCNamespace), s.IpcNS.Value); err != nil {
			return err
		}
	case specgen.Host:
		if err := g.RemoveLinuxNamespace(string(spec.IPCNamespace)); err != nil {
			return err
		}
	case specgen.Private:
		if err := g.AddOrReplaceLinuxNamespace(string(spec.IPCNamespace), ""); err != nil {
			return err
		}
	}

	// UTS
	switch s.UtsNS.NSMode {
	case specgen.Path:
		if _, err := os.Stat(s.UtsNS.Value); err != nil {
			return errors.Wrap(err, "cannot find specified UTS namespace path")
		}
		if err := g.AddOrReplaceLinuxNamespace(string(spec.UTSNamespace), s.UtsNS.Value); err != nil {
			return err
		}
	case specgen.Host:
		if err := g.RemoveLinuxNamespace(string(spec.UTSNamespace)); err != nil {
			return err
		}
	case specgen.Private:
		if err := g.AddOrReplaceLinuxNamespace(string(spec.UTSNamespace), ""); err != nil {
			return err
		}
	}

	hostname := s.Hostname
	if hostname == "" {
		switch {
		case s.UtsNS.NSMode == specgen.FromPod:
			hostname = pod.Hostname()
		case s.UtsNS.NSMode == specgen.FromContainer:
			utsCtr, err := rt.LookupContainer(s.UtsNS.Value)
			if err != nil {
				return errors.Wrapf(err, "error looking up container to share uts namespace with")
			}
			hostname = utsCtr.Hostname()
		case (s.NetNS.NSMode == specgen.Host && hostname == "") || s.UtsNS.NSMode == specgen.Host:
			tmpHostname, err := os.Hostname()
			if err != nil {
				return errors.Wrap(err, "unable to retrieve hostname of the host")
			}
			hostname = tmpHostname
		default:
			logrus.Debug("No hostname set; container's hostname will default to runtime default")
		}
	}

	g.RemoveHostname()
	if s.Hostname != "" || s.UtsNS.NSMode != specgen.Host {
		// Set the hostname in the OCI configuration only if specified by
		// the user or if we are creating a new UTS namespace.
		// TODO: Should we be doing this for pod or container shared
		// namespaces?
		g.SetHostname(hostname)
	}
	if _, ok := s.Env["HOSTNAME"]; !ok && s.Hostname != "" {
		g.AddProcessEnv("HOSTNAME", hostname)
	}

	// User
	if _, err := specgen.SetupUserNS(s.IDMappings, s.UserNS, g); err != nil {
		return err
	}

	// Cgroup
	switch s.CgroupNS.NSMode {
	case specgen.Path:
		if _, err := os.Stat(s.CgroupNS.Value); err != nil {
			return errors.Wrap(err, "cannot find specified cgroup namespace path")
		}
		if err := g.AddOrReplaceLinuxNamespace(string(spec.CgroupNamespace), s.CgroupNS.Value); err != nil {
			return err
		}
	case specgen.Host:
		if err := g.RemoveLinuxNamespace(string(spec.CgroupNamespace)); err != nil {
			return err
		}
	case specgen.Private:
		if err := g.AddOrReplaceLinuxNamespace(string(spec.CgroupNamespace), ""); err != nil {
			return err
		}
	}

	// Net
	switch s.NetNS.NSMode {
	case specgen.Path:
		if _, err := os.Stat(s.NetNS.Value); err != nil {
			return errors.Wrap(err, "cannot find specified network namespace path")
		}
		if err := g.AddOrReplaceLinuxNamespace(string(spec.NetworkNamespace), s.NetNS.Value); err != nil {
			return err
		}
	case specgen.Host:
		if err := g.RemoveLinuxNamespace(string(spec.NetworkNamespace)); err != nil {
			return err
		}
	case specgen.Private, specgen.NoNetwork:
		if err := g.AddOrReplaceLinuxNamespace(string(spec.NetworkNamespace), ""); err != nil {
			return err
		}
	}

	if g.Config.Annotations == nil {
		g.Config.Annotations = make(map[string]string)
	}
	if s.PublishExposedPorts {
		g.Config.Annotations[define.InspectAnnotationPublishAll] = define.InspectResponseTrue
	} else {
		g.Config.Annotations[define.InspectAnnotationPublishAll] = define.InspectResponseFalse
	}

	return nil
}

// GetNamespaceOptions transforms a slice of kernel namespaces
// into a slice of pod create options. Currently, not all
// kernel namespaces are supported, and they will be returned in an error
func GetNamespaceOptions(ns []string, netnsIsHost bool) ([]libpod.PodCreateOption, error) {
	var options []libpod.PodCreateOption
	var erroredOptions []libpod.PodCreateOption
	if ns == nil {
		// set the default namespaces
		ns = strings.Split(specgen.DefaultKernelNamespaces, ",")
	}
	for _, toShare := range ns {
		switch toShare {
		case "cgroup":
			options = append(options, libpod.WithPodCgroup())
		case "net":
			// share the netns setting with other containers in the pod only when it is not set to host
			if !netnsIsHost {
				options = append(options, libpod.WithPodNet())
			}
		case "mnt":
			return erroredOptions, errors.Errorf("Mount sharing functionality not supported on pod level")
		case "pid":
			options = append(options, libpod.WithPodPID())
		case "user":
			continue
		case "ipc":
			options = append(options, libpod.WithPodIPC())
		case "uts":
			options = append(options, libpod.WithPodUTS())
		case "":
		case "none":
			return erroredOptions, nil
		default:
			return erroredOptions, errors.Errorf("Invalid kernel namespace to share: %s. Options are: cgroup, ipc, net, pid, uts or none", toShare)
		}
	}
	return options, nil
}
