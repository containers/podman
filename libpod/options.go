package libpod

import (
	"net"
	"path/filepath"
	"regexp"
	"syscall"

	"github.com/containers/storage"
	"github.com/containers/storage/pkg/idtools"
	"github.com/cri-o/ocicni/pkg/ocicni"
	"github.com/pkg/errors"
)

var (
	nameRegex = regexp.MustCompile("[a-zA-Z0-9_-]+")
)

// Runtime Creation Options

// WithStorageConfig uses the given configuration to set up container storage
// If this is not specified, the system default configuration will be used
// instead
func WithStorageConfig(config storage.StoreOptions) RuntimeOption {
	return func(rt *Runtime) error {
		if rt.valid {
			return ErrRuntimeFinalized
		}

		rt.config.StorageConfig.RunRoot = config.RunRoot
		rt.config.StorageConfig.GraphRoot = config.GraphRoot
		rt.config.StorageConfig.GraphDriverName = config.GraphDriverName
		rt.config.StaticDir = filepath.Join(config.GraphRoot, "libpod")

		rt.config.StorageConfig.GraphDriverOptions = make([]string, len(config.GraphDriverOptions))
		copy(rt.config.StorageConfig.GraphDriverOptions, config.GraphDriverOptions)

		rt.config.StorageConfig.UIDMap = make([]idtools.IDMap, len(config.UIDMap))
		copy(rt.config.StorageConfig.UIDMap, config.UIDMap)

		rt.config.StorageConfig.GIDMap = make([]idtools.IDMap, len(config.GIDMap))
		copy(rt.config.StorageConfig.GIDMap, config.GIDMap)

		return nil
	}
}

// WithImageConfig uses the given configuration to set up image handling
// If this is not specified, the system default configuration will be used
// instead
func WithImageConfig(defaultTransport string, insecureRegistries, registries []string) RuntimeOption {
	return func(rt *Runtime) error {
		if rt.valid {
			return ErrRuntimeFinalized
		}

		rt.config.ImageDefaultTransport = defaultTransport

		rt.config.InsecureRegistries = make([]string, len(insecureRegistries))
		copy(rt.config.InsecureRegistries, insecureRegistries)

		rt.config.Registries = make([]string, len(registries))
		copy(rt.config.Registries, registries)

		return nil
	}
}

// WithSignaturePolicy specifies the path of a file which decides how trust is
// managed for images we've pulled.
// If this is not specified, the system default configuration will be used
// instead
func WithSignaturePolicy(path string) RuntimeOption {
	return func(rt *Runtime) error {
		if rt.valid {
			return ErrRuntimeFinalized
		}

		rt.config.SignaturePolicyPath = path

		return nil
	}
}

// WithStateType sets the backing state implementation for libpod
// Please note that information is not portable between backing states
// As such, if this differs between two libpods running on the same system,
// they will not share containers, and unspecified behavior may occur
func WithStateType(storeType RuntimeStateStore) RuntimeOption {
	return func(rt *Runtime) error {
		if rt.valid {
			return ErrRuntimeFinalized
		}

		if storeType == InvalidStateStore {
			return errors.Wrapf(ErrInvalidArg, "must provide a valid state store type")
		}

		rt.config.StateType = storeType

		return nil
	}
}

// WithOCIRuntime specifies an OCI runtime to use for running containers
func WithOCIRuntime(runtimePath string) RuntimeOption {
	return func(rt *Runtime) error {
		if rt.valid {
			return ErrRuntimeFinalized
		}

		rt.config.RuntimePath = runtimePath

		return nil
	}
}

// WithConmonPath specifies the path to the conmon binary which manages the
// runtime
func WithConmonPath(path string) RuntimeOption {
	return func(rt *Runtime) error {
		if rt.valid {
			return ErrRuntimeFinalized
		}
		// TODO Once libkpod is eliminated, "" should throw an error
		if path != "" {
			rt.config.ConmonPath = path
		}
		return nil
	}
}

// WithConmonEnv specifies the environment variable list for the conmon process
func WithConmonEnv(environment []string) RuntimeOption {
	return func(rt *Runtime) error {
		if rt.valid {
			return ErrRuntimeFinalized
		}

		rt.config.ConmonEnvVars = make([]string, len(environment))
		copy(rt.config.ConmonEnvVars, environment)

		return nil
	}
}

// WithCgroupManager specifies the manager implementation name which is used to
// handle cgroups for containers
// Current valid values are "cgroupfs" and "systemd"
func WithCgroupManager(manager string) RuntimeOption {
	return func(rt *Runtime) error {
		if rt.valid {
			return ErrRuntimeFinalized
		}

		rt.config.CgroupManager = manager

		return nil
	}
}

// WithStaticDir sets the directory that static runtime files which persist
// across reboots will be stored
func WithStaticDir(dir string) RuntimeOption {
	return func(rt *Runtime) error {
		if rt.valid {
			return ErrRuntimeFinalized
		}

		rt.config.StaticDir = dir

		return nil
	}
}

// WithTmpDir sets the directory that temporary runtime files which are not
// expected to survive across reboots will be stored
// This should be located on a tmpfs mount (/tmp or /var/run for example)
func WithTmpDir(dir string) RuntimeOption {
	return func(rt *Runtime) error {
		if rt.valid {
			return ErrRuntimeFinalized
		}

		rt.config.TmpDir = dir

		return nil
	}
}

// WithMaxLogSize sets the maximum size of container logs
// Positive sizes are limits in bytes, -1 is unlimited
func WithMaxLogSize(limit int64) RuntimeOption {
	return func(rt *Runtime) error {
		if rt.valid {
			return ErrRuntimeFinalized
		}

		rt.config.MaxLogSize = limit

		return nil
	}
}

// WithNoPivotRoot sets the runtime to use MS_MOVE instead of PIVOT_ROOT when
// starting containers
func WithNoPivotRoot(noPivot bool) RuntimeOption {
	return func(rt *Runtime) error {
		if rt.valid {
			return ErrRuntimeFinalized
		}

		rt.config.NoPivotRoot = true

		return nil
	}
}

// WithCNIConfigDir sets the CNI configuration directory
func WithCNIConfigDir(dir string) RuntimeOption {
	return func(rt *Runtime) error {
		if rt.valid {
			return ErrRuntimeFinalized
		}

		rt.config.CNIConfigDir = dir

		return nil
	}
}

// WithCNIPluginDir sets the CNI plugins directory
func WithCNIPluginDir(dir string) RuntimeOption {
	return func(rt *Runtime) error {
		if rt.valid {
			return ErrRuntimeFinalized
		}

		rt.config.CNIPluginDir = []string{dir}

		return nil
	}
}

// Container Creation Options

// WithShmDir sets the directory that should be mounted on /dev/shm
func WithShmDir(dir string) CtrCreateOption {
	return func(ctr *Container) error {
		if ctr.valid {
			return ErrCtrFinalized
		}

		ctr.config.ShmDir = dir
		return nil
	}
}

// WithShmSize sets the size of /dev/shm tmpfs mount
func WithShmSize(size int64) CtrCreateOption {
	return func(ctr *Container) error {
		if ctr.valid {
			return ErrCtrFinalized
		}

		ctr.config.ShmSize = size
		return nil
	}
}

// WithPrivileged sets the privileged flag in the container runtime
func WithPrivileged(privileged bool) CtrCreateOption {
	return func(ctr *Container) error {
		if ctr.valid {
			return ErrCtrFinalized
		}

		ctr.config.Privileged = privileged
		return nil
	}
}

// WithSELinuxLabels sets the mount label for SELinux
func WithSELinuxLabels(processLabel, mountLabel string) CtrCreateOption {
	return func(ctr *Container) error {
		if ctr.valid {
			return ErrCtrFinalized
		}

		ctr.config.ProcessLabel = processLabel
		ctr.config.MountLabel = mountLabel
		return nil
	}
}

// WithUser sets the user identity field in configutation
// Valid uses [user | user:group | uid | uid:gid | user:gid | uid:group ]
func WithUser(user string) CtrCreateOption {
	return func(ctr *Container) error {
		if ctr.valid {
			return ErrCtrFinalized
		}

		ctr.config.User = user
		return nil
	}
}

// WithRootFSFromImage sets up a fresh root filesystem using the given image
// If useImageConfig is specified, image volumes, environment variables, and
// other configuration from the image will be added to the config
// TODO: Replace image name and ID with a libpod.Image struct when that is finished
func WithRootFSFromImage(imageID string, imageName string, useImageVolumes bool) CtrCreateOption {
	return func(ctr *Container) error {
		if ctr.valid {
			return ErrCtrFinalized
		}

		if ctr.config.RootfsImageID != "" || ctr.config.RootfsImageName != "" {
			return errors.Wrapf(ErrInvalidArg, "container already configured with root filesystem")
		}

		ctr.config.RootfsImageID = imageID
		ctr.config.RootfsImageName = imageName
		ctr.config.ImageVolumes = useImageVolumes

		return nil
	}
}

// WithStdin keeps stdin on the container open to allow interaction
func WithStdin() CtrCreateOption {
	return func(ctr *Container) error {
		if ctr.valid {
			return ErrCtrFinalized
		}

		ctr.config.Stdin = true

		return nil
	}
}

// WithPod adds the container to a pod
func (r *Runtime) WithPod(pod *Pod) CtrCreateOption {
	return func(ctr *Container) error {
		if ctr.valid {
			return ErrCtrFinalized
		}

		if pod == nil {
			return ErrInvalidArg
		}

		ctr.config.Pod = pod.ID()

		return nil
	}
}

// WithLabels adds labels to the container
func WithLabels(labels map[string]string) CtrCreateOption {
	return func(ctr *Container) error {
		if ctr.valid {
			return ErrCtrFinalized
		}

		ctr.config.Labels = make(map[string]string)
		for key, value := range labels {
			ctr.config.Labels[key] = value
		}

		return nil
	}
}

// WithName sets the container's name
func WithName(name string) CtrCreateOption {
	return func(ctr *Container) error {
		if ctr.valid {
			return ErrCtrFinalized
		}

		// Check the name against a regex
		if !nameRegex.MatchString(name) {
			return errors.Wrapf(ErrInvalidArg, "name must match regex [a-zA-Z0-9_-]+")
		}

		ctr.config.Name = name

		return nil
	}
}

// WithStopSignal sets the signal that will be sent to stop the container
func WithStopSignal(signal syscall.Signal) CtrCreateOption {
	return func(ctr *Container) error {
		if ctr.valid {
			return ErrCtrFinalized
		}

		if signal == 0 {
			return errors.Wrapf(ErrInvalidArg, "stop signal cannot be 0")
		} else if signal > 64 {
			return errors.Wrapf(ErrInvalidArg, "stop signal cannot be greater than 64 (SIGRTMAX)")
		}

		ctr.config.StopSignal = uint(signal)

		return nil
	}
}

// WithStopTimeout sets the time to after initial stop signal is sent to container, before sending the kill signal
func WithStopTimeout(timeout uint) CtrCreateOption {
	return func(ctr *Container) error {
		if ctr.valid {
			return ErrCtrFinalized
		}

		ctr.config.StopTimeout = timeout

		return nil
	}
}

// WithIPCNSFrom indicates the the container should join the IPC namespace of
// the given container
func WithIPCNSFrom(nsCtr *Container) CtrCreateOption {
	return func(ctr *Container) error {
		if ctr.valid {
			return ErrCtrFinalized
		}

		if !nsCtr.valid {
			return ErrCtrRemoved
		}

		if nsCtr.ID() == ctr.ID() {
			return errors.Wrapf(ErrInvalidArg, "must specify another container")
		}

		ctr.config.IPCNsCtr = nsCtr.ID()

		return nil
	}
}

// WithMountNSFrom indicates the the container should join the mount namespace
// of the given container
func WithMountNSFrom(nsCtr *Container) CtrCreateOption {
	return func(ctr *Container) error {
		if ctr.valid {
			return ErrCtrFinalized
		}

		if !nsCtr.valid {
			return ErrCtrRemoved
		}

		if nsCtr.ID() == ctr.ID() {
			return errors.Wrapf(ErrInvalidArg, "must specify another container")
		}

		ctr.config.MountNsCtr = nsCtr.ID()

		return nil
	}
}

// WithNetNSFrom indicates the the container should join the network namespace
// of the given container
func WithNetNSFrom(nsCtr *Container) CtrCreateOption {
	return func(ctr *Container) error {
		if ctr.valid {
			return ErrCtrFinalized
		}

		if !nsCtr.valid {
			return ErrCtrRemoved
		}

		if nsCtr.ID() == ctr.ID() {
			return errors.Wrapf(ErrInvalidArg, "must specify another container")
		}

		if ctr.config.CreateNetNS {
			return errors.Wrapf(ErrInvalidArg, "cannot join another container's net ns as we are making a new net ns")
		}

		ctr.config.NetNsCtr = nsCtr.ID()

		return nil
	}
}

// WithPIDNSFrom indicates the the container should join the PID namespace of
// the given container
func WithPIDNSFrom(nsCtr *Container) CtrCreateOption {
	return func(ctr *Container) error {
		if ctr.valid {
			return ErrCtrFinalized
		}

		if !nsCtr.valid {
			return ErrCtrRemoved
		}

		if nsCtr.ID() == ctr.ID() {
			return errors.Wrapf(ErrInvalidArg, "must specify another container")
		}

		ctr.config.PIDNsCtr = nsCtr.ID()

		return nil
	}
}

// WithUserNSFrom indicates the the container should join the user namespace of
// the given container
func WithUserNSFrom(nsCtr *Container) CtrCreateOption {
	return func(ctr *Container) error {
		if ctr.valid {
			return ErrCtrFinalized
		}

		if !nsCtr.valid {
			return ErrCtrRemoved
		}

		if nsCtr.ID() == ctr.ID() {
			return errors.Wrapf(ErrInvalidArg, "must specify another container")
		}

		ctr.config.UserNsCtr = nsCtr.ID()

		return nil
	}
}

// WithUTSNSFrom indicates the the container should join the UTS namespace of
// the given container
func WithUTSNSFrom(nsCtr *Container) CtrCreateOption {
	return func(ctr *Container) error {
		if ctr.valid {
			return ErrCtrFinalized
		}

		if !nsCtr.valid {
			return ErrCtrRemoved
		}

		if nsCtr.ID() == ctr.ID() {
			return errors.Wrapf(ErrInvalidArg, "must specify another container")
		}

		ctr.config.UTSNsCtr = nsCtr.ID()

		return nil
	}
}

// WithCgroupNSFrom indicates the the container should join the CGroup namespace
// of the given container
func WithCgroupNSFrom(nsCtr *Container) CtrCreateOption {
	return func(ctr *Container) error {
		if ctr.valid {
			return ErrCtrFinalized
		}

		if !nsCtr.valid {
			return ErrCtrRemoved
		}

		if nsCtr.ID() == ctr.ID() {
			return errors.Wrapf(ErrInvalidArg, "must specify another container")
		}

		ctr.config.CgroupNsCtr = nsCtr.ID()

		return nil
	}
}

// WithNetNS indicates that the container should be given a new network
// namespace with a minimal configuration
// An optional array of port mappings can be provided
func WithNetNS(portMappings []ocicni.PortMapping) CtrCreateOption {
	return func(ctr *Container) error {
		if ctr.valid {
			return ErrCtrFinalized
		}

		if ctr.config.NetNsCtr != "" {
			return errors.Wrapf(ErrInvalidArg, "container is already set to join another container's net ns, cannot create a new net ns")
		}

		ctr.config.CreateNetNS = true
		ctr.config.PortMappings = portMappings

		return nil
	}
}

// WithCgroupParent sets the Cgroup Parent of the new container
func WithCgroupParent(parent string) CtrCreateOption {
	return func(ctr *Container) error {
		if ctr.valid {
			return ErrCtrFinalized
		}

		if parent == "" {
			return errors.Wrapf(ErrInvalidArg, "cgroup parent cannot be empty")
		}

		ctr.config.CgroupParent = parent

		return nil
	}
}

// Pod Creation Options

// WithPodName sets the name of the pod
func WithPodName(name string) PodCreateOption {
	return func(pod *Pod) error {
		if pod.valid {
			return ErrPodFinalized
		}

		// Check the name against a regex
		if !nameRegex.MatchString(name) {
			return errors.Wrapf(ErrInvalidArg, "name must match regex [a-zA-Z0-9_-]+")
		}

		pod.name = name

		return nil
	}
}

// WithPodLabels sets the labels of a pod
func WithPodLabels(labels map[string]string) PodCreateOption {
	return func(pod *Pod) error {
		if pod.valid {
			return ErrPodFinalized
		}

		pod.labels = make(map[string]string)
		for key, value := range labels {
			pod.labels[key] = value
		}

		return nil
	}
}

// WithDNSSearch sets the additional search domains of a container
func WithDNSSearch(searchDomains []string) CtrCreateOption {
	return func(ctr *Container) error {
		if ctr.valid {
			return ErrCtrFinalized
		}
		ctr.config.DNSSearch = searchDomains
		return nil
	}
}

// WithDNS sets additional name servers for the container
func WithDNS(dnsServers []string) CtrCreateOption {
	return func(ctr *Container) error {
		if ctr.valid {
			return ErrCtrFinalized
		}
		var dns []net.IP
		for _, i := range dnsServers {
			result := net.ParseIP(i)
			if result == nil {
				return errors.Wrapf(ErrInvalidArg, "invalid IP address %s", i)
			}
			dns = append(dns, result)
		}
		ctr.config.DNSServer = dns
		return nil
	}
}

// WithDNSOption sets addition dns options for the container
func WithDNSOption(dnsOptions []string) CtrCreateOption {
	return func(ctr *Container) error {
		if ctr.valid {
			return ErrCtrFinalized
		}
		ctr.config.DNSOption = dnsOptions
		return nil
	}
}

// WithHosts sets additional host:IP for the hosts file
func WithHosts(hosts []string) CtrCreateOption {
	return func(ctr *Container) error {
		if ctr.valid {
			return ErrCtrFinalized
		}
		ctr.config.HostAdd = hosts
		return nil
	}
}
