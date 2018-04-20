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

// WithStorageConfig uses the given configuration to set up container storage.
// If this is not specified, the system default configuration will be used
// instead.
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

// WithDefaultTransport sets the default transport for retrieving images.
func WithDefaultTransport(defaultTransport string) RuntimeOption {
	return func(rt *Runtime) error {
		if rt.valid {
			return ErrRuntimeFinalized
		}

		rt.config.ImageDefaultTransport = defaultTransport

		return nil
	}
}

// WithSignaturePolicy specifies the path of a file which decides how trust is
// managed for images we've pulled.
// If this is not specified, the system default configuration will be used
// instead.
func WithSignaturePolicy(path string) RuntimeOption {
	return func(rt *Runtime) error {
		if rt.valid {
			return ErrRuntimeFinalized
		}

		rt.config.SignaturePolicyPath = path

		return nil
	}
}

// WithStateType sets the backing state implementation for libpod.
// Please note that information is not portable between backing states.
// As such, if this differs between two libpods running on the same system,
// they will not share containers, and unspecified behavior may occur.
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

// WithOCIRuntime specifies an OCI runtime to use for running containers.
func WithOCIRuntime(runtimePath string) RuntimeOption {
	return func(rt *Runtime) error {
		if rt.valid {
			return ErrRuntimeFinalized
		}

		if runtimePath == "" {
			return errors.Wrapf(ErrInvalidArg, "must provide a valid path")
		}

		rt.config.RuntimePath = []string{runtimePath}

		return nil
	}
}

// WithConmonPath specifies the path to the conmon binary which manages the
// runtime.
func WithConmonPath(path string) RuntimeOption {
	return func(rt *Runtime) error {
		if rt.valid {
			return ErrRuntimeFinalized
		}

		if path == "" {
			return errors.Wrapf(ErrInvalidArg, "must provide a valid path")
		}

		rt.config.ConmonPath = []string{path}

		return nil
	}
}

// WithConmonEnv specifies the environment variable list for the conmon process.
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
// handle cgroups for containers.
// Current valid values are "cgroupfs" and "systemd".
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
// across reboots will be stored.
func WithStaticDir(dir string) RuntimeOption {
	return func(rt *Runtime) error {
		if rt.valid {
			return ErrRuntimeFinalized
		}

		rt.config.StaticDir = dir

		return nil
	}
}

// WithHooksDir sets the directory to look for OCI runtime hooks config.
// Note we are not saving this in database, since this is really just for used
// for testing.
func WithHooksDir(hooksDir string) RuntimeOption {
	return func(rt *Runtime) error {
		if rt.valid {
			return ErrRuntimeFinalized
		}

		rt.config.HooksDir = hooksDir
		return nil
	}
}

// WithDefaultMountsFile sets the file to look at for default mounts (mainly
// secrets).
// Note we are not saving this in the database as it is for testing purposes
// only.
func WithDefaultMountsFile(mountsFile string) RuntimeOption {
	return func(rt *Runtime) error {
		if rt.valid {
			return ErrRuntimeFinalized
		}

		if mountsFile == "" {
			return ErrInvalidArg
		}
		rt.config.DefaultMountsFile = mountsFile
		return nil
	}
}

// WithTmpDir sets the directory that temporary runtime files which are not
// expected to survive across reboots will be stored.
// This should be located on a tmpfs mount (/tmp or /var/run for example).
func WithTmpDir(dir string) RuntimeOption {
	return func(rt *Runtime) error {
		if rt.valid {
			return ErrRuntimeFinalized
		}

		rt.config.TmpDir = dir

		return nil
	}
}

// WithMaxLogSize sets the maximum size of container logs.
// Positive sizes are limits in bytes, -1 is unlimited.
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
// starting containers.
func WithNoPivotRoot(noPivot bool) RuntimeOption {
	return func(rt *Runtime) error {
		if rt.valid {
			return ErrRuntimeFinalized
		}

		rt.config.NoPivotRoot = true

		return nil
	}
}

// WithCNIConfigDir sets the CNI configuration directory.
func WithCNIConfigDir(dir string) RuntimeOption {
	return func(rt *Runtime) error {
		if rt.valid {
			return ErrRuntimeFinalized
		}

		rt.config.CNIConfigDir = dir

		return nil
	}
}

// WithCNIPluginDir sets the CNI plugins directory.
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

// WithShmDir sets the directory that should be mounted on /dev/shm.
func WithShmDir(dir string) CtrCreateOption {
	return func(ctr *Container) error {
		if ctr.valid {
			return ErrCtrFinalized
		}

		ctr.config.ShmDir = dir
		return nil
	}
}

// WithShmSize sets the size of /dev/shm tmpfs mount.
func WithShmSize(size int64) CtrCreateOption {
	return func(ctr *Container) error {
		if ctr.valid {
			return ErrCtrFinalized
		}

		ctr.config.ShmSize = size
		return nil
	}
}

// WithPrivileged sets the privileged flag in the container runtime.
func WithPrivileged(privileged bool) CtrCreateOption {
	return func(ctr *Container) error {
		if ctr.valid {
			return ErrCtrFinalized
		}

		ctr.config.Privileged = privileged
		return nil
	}
}

// WithSELinuxLabels sets the mount label for SELinux.
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

// WithUser sets the user identity field in configutation.
// Valid uses [user | user:group | uid | uid:gid | user:gid | uid:group ].
func WithUser(user string) CtrCreateOption {
	return func(ctr *Container) error {
		if ctr.valid {
			return ErrCtrFinalized
		}

		ctr.config.User = user
		return nil
	}
}

// WithRootFSFromImage sets up a fresh root filesystem using the given image.
// If useImageConfig is specified, image volumes, environment variables, and
// other configuration from the image will be added to the config.
// TODO: Replace image name and ID with a libpod.Image struct when that is
// finished.
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

// WithStdin keeps stdin on the container open to allow interaction.
func WithStdin() CtrCreateOption {
	return func(ctr *Container) error {
		if ctr.valid {
			return ErrCtrFinalized
		}

		ctr.config.Stdin = true

		return nil
	}
}

// WithPod adds the container to a pod.
// Containers which join a pod can only join the namespaces of other containers
// in the same pod.
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

// WithLabels adds labels to the container.
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

// WithName sets the container's name.
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

// WithStopSignal sets the signal that will be sent to stop the container.
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

// WithStopTimeout sets the time to after initial stop signal is sent to the
// container, before sending the kill signal.
func WithStopTimeout(timeout uint) CtrCreateOption {
	return func(ctr *Container) error {
		if ctr.valid {
			return ErrCtrFinalized
		}

		ctr.config.StopTimeout = timeout

		return nil
	}
}

// WithIDMappings sets the idmappsings for the container
func WithIDMappings(idmappings storage.IDMappingOptions) CtrCreateOption {
	return func(ctr *Container) error {
		if ctr.valid {
			return ErrCtrFinalized
		}

		ctr.config.IDMappings = idmappings
		return nil
	}
}

// WithIPCNSFrom indicates the the container should join the IPC namespace of
// the given container.
// If the container has joined a pod, it can only join the namespaces of
// containers in the same pod.
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

		if ctr.config.Pod != "" && nsCtr.config.Pod != ctr.config.Pod {
			return errors.Wrapf(ErrInvalidArg, "container has joined pod %s and dependency container %s is not a member of the pod", ctr.config.Pod, nsCtr.ID())
		}

		ctr.config.IPCNsCtr = nsCtr.ID()

		return nil
	}
}

// WithMountNSFrom indicates the the container should join the mount namespace
// of the given container.
// If the container has joined a pod, it can only join the namespaces of
// containers in the same pod.
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

		if ctr.config.Pod != "" && nsCtr.config.Pod != ctr.config.Pod {
			return errors.Wrapf(ErrInvalidArg, "container has joined pod %s and dependency container %s is not a member of the pod", ctr.config.Pod, nsCtr.ID())
		}

		ctr.config.MountNsCtr = nsCtr.ID()

		return nil
	}
}

// WithNetNSFrom indicates the the container should join the network namespace
// of the given container.
// If the container has joined a pod, it can only join the namespaces of
// containers in the same pod.
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

		if ctr.config.Pod != "" && nsCtr.config.Pod != ctr.config.Pod {
			return errors.Wrapf(ErrInvalidArg, "container has joined pod %s and dependency container %s is not a member of the pod", ctr.config.Pod, nsCtr.ID())
		}

		ctr.config.NetNsCtr = nsCtr.ID()

		return nil
	}
}

// WithPIDNSFrom indicates the the container should join the PID namespace of
// the given container.
// If the container has joined a pod, it can only join the namespaces of
// containers in the same pod.
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

		if ctr.config.Pod != "" && nsCtr.config.Pod != ctr.config.Pod {
			return errors.Wrapf(ErrInvalidArg, "container has joined pod %s and dependency container %s is not a member of the pod", ctr.config.Pod, nsCtr.ID())
		}

		ctr.config.PIDNsCtr = nsCtr.ID()

		return nil
	}
}

// WithUserNSFrom indicates the the container should join the user namespace of
// the given container.
// If the container has joined a pod, it can only join the namespaces of
// containers in the same pod.
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

		if ctr.config.Pod != "" && nsCtr.config.Pod != ctr.config.Pod {
			return errors.Wrapf(ErrInvalidArg, "container has joined pod %s and dependency container %s is not a member of the pod", ctr.config.Pod, nsCtr.ID())
		}

		ctr.config.UserNsCtr = nsCtr.ID()

		return nil
	}
}

// WithUTSNSFrom indicates the the container should join the UTS namespace of
// the given container.
// If the container has joined a pod, it can only join the namespaces of
// containers in the same pod.
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

		if ctr.config.Pod != "" && nsCtr.config.Pod != ctr.config.Pod {
			return errors.Wrapf(ErrInvalidArg, "container has joined pod %s and dependency container %s is not a member of the pod", ctr.config.Pod, nsCtr.ID())
		}

		ctr.config.UTSNsCtr = nsCtr.ID()

		return nil
	}
}

// WithCgroupNSFrom indicates the the container should join the CGroup namespace
// of the given container.
// If the container has joined a pod, it can only join the namespaces of
// containers in the same pod.
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

		if ctr.config.Pod != "" && nsCtr.config.Pod != ctr.config.Pod {
			return errors.Wrapf(ErrInvalidArg, "container has joined pod %s and dependency container %s is not a member of the pod", ctr.config.Pod, nsCtr.ID())
		}

		ctr.config.CgroupNsCtr = nsCtr.ID()

		return nil
	}
}

// WithDependencyCtrs sets dependency containers of the given container.
// Dependency containers must be running before this container is started.
func WithDependencyCtrs(ctrs []*Container) CtrCreateOption {
	return func(ctr *Container) error {
		if ctr.valid {
			return ErrCtrFinalized
		}

		deps := make([]string, 0, len(ctrs))

		for _, dep := range ctrs {
			if !dep.valid {
				return errors.Wrapf(ErrCtrRemoved, "container %s is not valid", dep.ID())
			}

			if dep.ID() == ctr.ID() {
				return errors.Wrapf(ErrInvalidArg, "must specify another container")
			}

			if ctr.config.Pod != "" && dep.config.Pod != ctr.config.Pod {
				return errors.Wrapf(ErrInvalidArg, "container has joined pod %s and dependency container %s is not a member of the pod", ctr.config.Pod, dep.ID())
			}

			deps = append(deps, dep.ID())
		}

		ctr.config.Dependencies = deps

		return nil
	}
}

// WithNetNS indicates that the container should be given a new network
// namespace with a minimal configuration.
// An optional array of port mappings can be provided.
// Conflicts with WithNetNSFrom().
func WithNetNS(portMappings []ocicni.PortMapping, postConfigureNetNS bool) CtrCreateOption {
	return func(ctr *Container) error {
		if ctr.valid {
			return ErrCtrFinalized
		}

		if ctr.config.NetNsCtr != "" {
			return errors.Wrapf(ErrInvalidArg, "container is already set to join another container's net ns, cannot create a new net ns")
		}

		ctr.config.PostConfigureNetNS = postConfigureNetNS
		ctr.config.CreateNetNS = true
		ctr.config.PortMappings = portMappings

		return nil
	}
}

// WithLogPath sets the path to the log file.
func WithLogPath(path string) CtrCreateOption {
	return func(ctr *Container) error {
		if ctr.valid {
			return ErrCtrFinalized
		}
		if path == "" {
			return errors.Wrapf(ErrInvalidArg, "log path must be set")
		}

		ctr.config.LogPath = path

		return nil
	}
}

// WithCgroupParent sets the Cgroup Parent of the new container.
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

// WithDNSSearch sets the additional search domains of a container.
func WithDNSSearch(searchDomains []string) CtrCreateOption {
	return func(ctr *Container) error {
		if ctr.valid {
			return ErrCtrFinalized
		}
		ctr.config.DNSSearch = searchDomains
		return nil
	}
}

// WithDNS sets additional name servers for the container.
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

// WithDNSOption sets addition dns options for the container.
func WithDNSOption(dnsOptions []string) CtrCreateOption {
	return func(ctr *Container) error {
		if ctr.valid {
			return ErrCtrFinalized
		}
		ctr.config.DNSOption = dnsOptions
		return nil
	}
}

// WithHosts sets additional host:IP for the hosts file.
func WithHosts(hosts []string) CtrCreateOption {
	return func(ctr *Container) error {
		if ctr.valid {
			return ErrCtrFinalized
		}
		ctr.config.HostAdd = hosts
		return nil
	}
}

// WithConmonPidFile specifies the path to the file that receives the pid of
// conmon.
func WithConmonPidFile(path string) CtrCreateOption {
	return func(ctr *Container) error {
		if ctr.valid {
			return ErrCtrFinalized
		}
		ctr.config.ConmonPidFile = path
		return nil
	}
}

// WithGroups sets additional groups for the container, which are defined by
// the user.
func WithGroups(groups []string) CtrCreateOption {
	return func(ctr *Container) error {
		if ctr.valid {
			return ErrCtrFinalized
		}
		ctr.config.Groups = groups
		return nil
	}
}

// WithUserVolumes sets the user-added volumes of the container.
// These are not added to the container's spec, but will instead be used during
// commit to populate the volumes of the new image, and to trigger some OCI
// hooks that are only added if volume mounts are present.
// Unless explicitly set, committed images will have no volumes.
// The given volumes slice must not be nil.
func WithUserVolumes(volumes []string) CtrCreateOption {
	return func(ctr *Container) error {
		if ctr.valid {
			return ErrCtrFinalized
		}

		if volumes == nil {
			return ErrInvalidArg
		}

		ctr.config.UserVolumes = make([]string, 0, len(volumes))
		for _, vol := range volumes {
			ctr.config.UserVolumes = append(ctr.config.UserVolumes, vol)
		}

		return nil
	}
}

// WithEntrypoint sets the entrypoint of the container.
// This is not used to change the container's spec, but will instead be used
// during commit to populate the entrypoint of the new image.
// If not explicitly set it will default to the image's entrypoint.
// A nil entrypoint is allowed, and will clear entrypoint on the created image.
func WithEntrypoint(entrypoint []string) CtrCreateOption {
	return func(ctr *Container) error {
		if ctr.valid {
			return ErrCtrFinalized
		}

		ctr.config.Entrypoint = make([]string, 0, len(entrypoint))
		for _, str := range entrypoint {
			ctr.config.Entrypoint = append(ctr.config.Entrypoint, str)
		}

		return nil
	}
}

// WithCommand sets the command of the container.
// This is not used to change the container's spec, but will instead be used
// during commit to populate the command of the new image.
// If not explicitly set it will default to the image's command.
// A nil command is allowed, and will clear command on the created image.
func WithCommand(command []string) CtrCreateOption {
	return func(ctr *Container) error {
		if ctr.valid {
			return ErrCtrFinalized
		}

		ctr.config.Command = make([]string, 0, len(command))
		for _, str := range command {
			ctr.config.Command = append(ctr.config.Command, str)
		}

		return nil
	}
}

// Pod Creation Options

// WithPodName sets the name of the pod.
func WithPodName(name string) PodCreateOption {
	return func(pod *Pod) error {
		if pod.valid {
			return ErrPodFinalized
		}

		// Check the name against a regex
		if !nameRegex.MatchString(name) {
			return errors.Wrapf(ErrInvalidArg, "name must match regex [a-zA-Z0-9_-]+")
		}

		pod.config.Name = name

		return nil
	}
}

// WithPodLabels sets the labels of a pod.
func WithPodLabels(labels map[string]string) PodCreateOption {
	return func(pod *Pod) error {
		if pod.valid {
			return ErrPodFinalized
		}

		pod.config.Labels = make(map[string]string)
		for key, value := range labels {
			pod.config.Labels[key] = value
		}

		return nil
	}
}
