package config

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/BurntSushi/toml"
	"github.com/containers/common/pkg/capabilities"
	"github.com/containers/storage/pkg/unshare"
	units "github.com/docker/go-units"
	selinux "github.com/opencontainers/selinux/go-selinux"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	// _configPath is the path to the containers/containers.conf
	// inside a given config directory.
	_configPath = "containers/containers.conf"
	// DefaultContainersConfig holds the default containers config path
	DefaultContainersConfig = "/usr/share/" + _configPath
	// OverrideContainersConfig holds the default config path overridden by the root user
	OverrideContainersConfig = "/etc/" + _configPath
	// UserOverrideContainersConfig holds the containers config path overridden by the rootless user
	UserOverrideContainersConfig = ".config/" + _configPath
)

// RuntimeStateStore is a constant indicating which state store implementation
// should be used by engine
type RuntimeStateStore int

const (
	// InvalidStateStore is an invalid state store
	InvalidStateStore RuntimeStateStore = iota
	// InMemoryStateStore is an in-memory state that will not persist data
	// on containers and pods between engine instances or after system
	// reboot
	InMemoryStateStore RuntimeStateStore = iota
	// SQLiteStateStore is a state backed by a SQLite database
	// It is presently disabled
	SQLiteStateStore RuntimeStateStore = iota
	// BoltDBStateStore is a state backed by a BoltDB database
	BoltDBStateStore RuntimeStateStore = iota
)

// Config contains configuration options for container tools
type Config struct {
	// Containers specify settings that configure how containers will run ont the system
	Containers ContainersConfig `toml:"containers"`
	// Engine specifies how the container engine based on Engine will run
	Engine EngineConfig `toml:"engine"`
	// Network section defines the configuration of CNI Plugins
	Network NetworkConfig `toml:"network"`
	// Secret section defines configurations for the secret management
	Secrets SecretConfig `toml:"secrets"`
}

// ContainersConfig represents the "containers" TOML config table
// containers global options for containers tools
type ContainersConfig struct {

	// Devices to add to all containers
	Devices []string `toml:"devices,omitempty"`

	// Volumes to add to all containers
	Volumes []string `toml:"volumes,omitempty"`

	// ApparmorProfile is the apparmor profile name which is used as the
	// default for the runtime.
	ApparmorProfile string `toml:"apparmor_profile,omitempty"`

	// Annotation to add to all containers
	Annotations []string `toml:"annotations,omitempty"`

	// Default way to create a cgroup namespace for the container
	CgroupNS string `toml:"cgroupns,omitempty"`

	// Default cgroup configuration
	Cgroups string `toml:"cgroups,omitempty"`

	// Capabilities to add to all containers.
	DefaultCapabilities []string `toml:"default_capabilities,omitempty"`

	// Sysctls to add to all containers.
	DefaultSysctls []string `toml:"default_sysctls,omitempty"`

	// DefaultUlimits specifies the default ulimits to apply to containers
	DefaultUlimits []string `toml:"default_ulimits,omitempty"`

	// DefaultMountsFile is the path to the default mounts file for testing
	DefaultMountsFile string `toml:"-"`

	// DNSServers set default DNS servers.
	DNSServers []string `toml:"dns_servers,omitempty"`

	// DNSOptions set default DNS options.
	DNSOptions []string `toml:"dns_options,omitempty"`

	// DNSSearches set default DNS search domains.
	DNSSearches []string `toml:"dns_searches,omitempty"`

	// EnableKeyring tells the container engines whether to create
	// a kernel keyring for use within the container
	EnableKeyring bool `toml:"keyring,omitempty"`

	// EnableLabeling tells the container engines whether to use MAC
	// Labeling to separate containers (SELinux)
	EnableLabeling bool `toml:"label,omitempty"`

	// Env is the environment variable list for container process.
	Env []string `toml:"env,omitempty"`

	// EnvHost Pass all host environment variables into the container.
	EnvHost bool `toml:"env_host,omitempty"`

	// HTTPProxy is the proxy environment variable list to apply to container process
	HTTPProxy bool `toml:"http_proxy,omitempty"`

	// Init tells container runtimes whether to run init inside the
	// container that forwards signals and reaps processes.
	Init bool `toml:"init,omitempty"`

	// InitPath is the path for init to run if the Init bool is enabled
	InitPath string `toml:"init_path,omitempty"`

	// IPCNS way to to create a ipc namespace for the container
	IPCNS string `toml:"ipcns,omitempty"`

	// LogDriver  for the container.  For example: k8s-file and journald
	LogDriver string `toml:"log_driver,omitempty"`

	// LogSizeMax is the maximum number of bytes after which the log file
	// will be truncated. It can be expressed as a human-friendly string
	// that is parsed to bytes.
	// Negative values indicate that the log file won't be truncated.
	LogSizeMax int64 `toml:"log_size_max,omitempty"`

	// Specifies default format tag for container log messages.
	// This is useful for creating a specific tag for container log messages.
	// Containers logs default to truncated container ID as a tag.
	LogTag string `toml:"log_tag,omitempty"`

	// NetNS indicates how to create a network namespace for the container
	NetNS string `toml:"netns,omitempty"`

	// NoHosts tells container engine whether to create its own /etc/hosts
	NoHosts bool `toml:"no_hosts,omitempty"`

	// PidsLimit is the number of processes each container is restricted to
	// by the cgroup process number controller.
	PidsLimit int64 `toml:"pids_limit,omitempty"`

	// PidNS indicates how to create a pid namespace for the container
	PidNS string `toml:"pidns,omitempty"`

	// Copy the content from the underlying image into the newly created
	// volume when the container is created instead of when it is started.
	// If false, the container engine will not copy the content until
	// the container is started. Setting it to true may have negative
	// performance implications.
	PrepareVolumeOnCreate bool `toml:"prepare_volume_on_create,omitempty"`

	// RootlessNetworking depicts the "kind" of networking for rootless
	// containers.  Valid options are `slirp4netns` and `cni`. Default is
	// `slirp4netns` on Linux, and `cni` on non-Linux OSes.
	RootlessNetworking string `toml:"rootless_networking,omitempty"`

	// SeccompProfile is the seccomp.json profile path which is used as the
	// default for the runtime.
	SeccompProfile string `toml:"seccomp_profile,omitempty"`

	// ShmSize holds the size of /dev/shm.
	ShmSize string `toml:"shm_size,omitempty"`

	// TZ sets the timezone inside the container
	TZ string `toml:"tz,omitempty"`

	// Umask is the umask inside the container.
	Umask string `toml:"umask,omitempty"`

	// UTSNS indicates how to create a UTS namespace for the container
	UTSNS string `toml:"utsns,omitempty"`

	// UserNS indicates how to create a User namespace for the container
	UserNS string `toml:"userns,omitempty"`

	// UserNSSize how many UIDs to allocate for automatically created UserNS
	UserNSSize int `toml:"userns_size,omitempty"`
}

// EngineConfig contains configuration options used to set up a engine runtime
type EngineConfig struct {
	// CgroupCheck indicates the configuration has been rewritten after an
	// upgrade to Fedora 31 to change the default OCI runtime for cgroupv2v2.
	CgroupCheck bool `toml:"cgroup_check,omitempty"`

	// CGroupManager is the CGroup Manager to use Valid values are "cgroupfs"
	// and "systemd".
	CgroupManager string `toml:"cgroup_manager,omitempty"`

	// NOTE: when changing this struct, make sure to update (*Config).Merge().

	// ConmonEnvVars are environment variables to pass to the Conmon binary
	// when it is launched.
	ConmonEnvVars []string `toml:"conmon_env_vars,omitempty"`

	// ConmonPath is the path to the Conmon binary used for managing containers.
	// The first path pointing to a valid file will be used.
	ConmonPath []string `toml:"conmon_path,omitempty"`

	// DetachKeys is the sequence of keys used to detach a container.
	DetachKeys string `toml:"detach_keys,omitempty"`

	// EnablePortReservation determines whether engine will reserve ports on the
	// host when they are forwarded to containers. When enabled, when ports are
	// forwarded to containers, they are held open by conmon as long as the
	// container is running, ensuring that they cannot be reused by other
	// programs on the host. However, this can cause significant memory usage if
	// a container has many ports forwarded to it. Disabling this can save
	// memory.
	EnablePortReservation bool `toml:"enable_port_reservation,omitempty"`

	// Environment variables to be used when running the container engine (e.g., Podman, Buildah). For example "http_proxy=internal.proxy.company.com"
	Env []string `toml:"env,omitempty"`

	// EventsLogFilePath is where the events log is stored.
	EventsLogFilePath string `toml:"events_logfile_path,omitempty"`

	// EventsLogger determines where events should be logged.
	EventsLogger string `toml:"events_logger,omitempty"`

	// HelperBinariesDir is a list of directories which are used to search for
	// helper binaries.
	HelperBinariesDir []string `toml:"helper_binaries_dir"`

	// configuration files. When the same filename is present in in
	// multiple directories, the file in the directory listed last in
	// this slice takes precedence.
	HooksDir []string `toml:"hooks_dir,omitempty"`

	// ImageBuildFormat (DEPRECATED) indicates the default image format to
	// building container images. Should use ImageDefaultFormat
	ImageBuildFormat string `toml:"image_build_format,omitempty"`

	// ImageDefaultTransport is the default transport method used to fetch
	// images.
	ImageDefaultTransport string `toml:"image_default_transport,omitempty"`

	// ImageParallelCopies indicates the maximum number of image layers
	// to be copied simultaneously. If this is zero, container engines
	// will fall back to containers/image defaults.
	ImageParallelCopies uint `toml:"image_parallel_copies,omitempty"`

	// ImageDefaultFormat specified the manifest Type (oci, v2s2, or v2s1)
	// to use when pulling, pushing, building container images. By default
	// image pulled and pushed match the format of the source image.
	// Building/committing defaults to OCI.
	ImageDefaultFormat string `toml:"image_default_format,omitempty"`

	// InfraCommand is the command run to start up a pod infra container.
	InfraCommand string `toml:"infra_command,omitempty"`

	// InfraImage is the image a pod infra container will use to manage
	// namespaces.
	InfraImage string `toml:"infra_image,omitempty"`

	// InitPath is the path to the container-init binary.
	InitPath string `toml:"init_path,omitempty"`

	// LockType is the type of locking to use.
	LockType string `toml:"lock_type,omitempty"`

	// MachineEnabled indicates if Podman is running in a podman-machine VM
	MachineEnabled bool `toml:"machine_enabled,omitempty"`

	// MachineImage is the image used when creating a podman-machine VM
	MachineImage string `toml:"machine_image,omitempty"`

	// MultiImageArchive - if true, the container engine allows for storing
	// archives (e.g., of the docker-archive transport) with multiple
	// images.  By default, Podman creates single-image archives.
	MultiImageArchive bool `toml:"multi_image_archive,omitempty"`

	// Namespace is the engine namespace to use. Namespaces are used to create
	// scopes to separate containers and pods in the state. When namespace is
	// set, engine will only view containers and pods in the same namespace. All
	// containers and pods created will default to the namespace set here. A
	// namespace of "", the empty string, is equivalent to no namespace, and all
	// containers and pods will be visible. The default namespace is "".
	Namespace string `toml:"namespace,omitempty"`

	// NetworkCmdPath is the path to the slirp4netns binary.
	NetworkCmdPath string `toml:"network_cmd_path,omitempty"`

	// NetworkCmdOptions is the default options to pass to the slirp4netns binary.
	// For example "allow_host_loopback=true"
	NetworkCmdOptions []string `toml:"network_cmd_options,omitempty"`

	// NoPivotRoot sets whether to set no-pivot-root in the OCI runtime.
	NoPivotRoot bool `toml:"no_pivot_root,omitempty"`

	// NumLocks is the number of locks to make available for containers and
	// pods.
	NumLocks uint32 `toml:"num_locks,omitempty"`

	// OCIRuntime is the OCI runtime to use.
	OCIRuntime string `toml:"runtime,omitempty"`

	// OCIRuntimes are the set of configured OCI runtimes (default is runc).
	OCIRuntimes map[string][]string `toml:"runtimes,omitempty"`

	// PullPolicy determines whether to pull image before creating or running a container
	// default is "missing"
	PullPolicy string `toml:"pull_policy,omitempty"`

	// Indicates whether the application should be running in Remote mode
	Remote bool `toml:"remote,omitempty"`

	// RemoteURI is deprecated, see ActiveService
	// RemoteURI containers connection information used to connect to remote system.
	RemoteURI string `toml:"remote_uri,omitempty"`

	// RemoteIdentity is deprecated, ServiceDestinations
	// RemoteIdentity key file for RemoteURI
	RemoteIdentity string `toml:"remote_identity,omitempty"`

	// ActiveService index to Destinations added v2.0.3
	ActiveService string `toml:"active_service,omitempty"`

	// Destinations mapped by service Names
	ServiceDestinations map[string]Destination `toml:"service_destinations,omitempty"`

	// RuntimePath is the path to OCI runtime binary for launching containers.
	// The first path pointing to a valid file will be used This is used only
	// when there are no OCIRuntime/OCIRuntimes defined.  It is used only to be
	// backward compatible with older versions of Podman.
	RuntimePath []string `toml:"runtime_path,omitempty"`

	// RuntimeSupportsJSON is the list of the OCI runtimes that support
	// --format=json.
	RuntimeSupportsJSON []string `toml:"runtime_supports_json,omitempty"`

	// RuntimeSupportsNoCgroups is a list of OCI runtimes that support
	// running containers without CGroups.
	RuntimeSupportsNoCgroups []string `toml:"runtime_supports_nocgroup,omitempty"`

	// RuntimeSupportsKVM is a list of OCI runtimes that support
	// KVM separation for containers.
	RuntimeSupportsKVM []string `toml:"runtime_supports_kvm,omitempty"`

	// SetOptions contains a subset of config options. It's used to indicate if
	// a given option has either been set by the user or by the parsed
	// configuration file. If not, the corresponding option might be
	// overwritten by values from the database. This behavior guarantees
	// backwards compat with older version of libpod and Podman.
	SetOptions

	// SignaturePolicyPath is the path to a signature policy to use for
	// validating images. If left empty, the containers/image default signature
	// policy will be used.
	SignaturePolicyPath string `toml:"-"`

	// SDNotify tells container engine to allow containers to notify the host systemd of
	// readiness using the SD_NOTIFY mechanism.
	SDNotify bool `toml:"-"`

	// StateType is the type of the backing state store. Avoid using multiple
	// values for this with the same containers/storage configuration on the
	// same system. Different state types do not interact, and each will see a
	// separate set of containers, which may cause conflicts in
	// containers/storage. As such this is not exposed via the config file.
	StateType RuntimeStateStore `toml:"-"`

	// StaticDir is the path to a persistent directory to store container
	// files.
	StaticDir string `toml:"static_dir,omitempty"`

	// StopTimeout is the number of seconds to wait for container to exit
	// before sending kill signal.
	StopTimeout uint `toml:"stop_timeout,omitempty"`

	// TmpDir is the path to a temporary directory to store per-boot container
	// files. Must be stored in a tmpfs.
	TmpDir string `toml:"tmp_dir,omitempty"`

	// VolumePath is the default location that named volumes will be created
	// under. This convention is followed by the default volume driver, but
	// may not be by other drivers.
	VolumePath string `toml:"volume_path,omitempty"`

	// VolumePlugins is a set of plugins that can be used as the backend for
	// Podman named volumes. Each volume is specified as a name (what Podman
	// will refer to the plugin as) mapped to a path, which must point to a
	// Unix socket that conforms to the Volume Plugin specification.
	VolumePlugins map[string]string `toml:"volume_plugins,omitempty"`

	// ChownCopiedFiles tells the container engine whether to chown files copied
	// into a container to the container's primary uid/gid.
	ChownCopiedFiles bool `toml:"chown_copied_files"`
}

// SetOptions contains a subset of options in a Config. It's used to indicate if
// a given option has either been set by the user or by a parsed engine
// configuration file. If not, the corresponding option might be overwritten by
// values from the database. This behavior guarantees backwards compat with
// older version of libpod and Podman.
type SetOptions struct {
	// StorageConfigRunRootSet indicates if the RunRoot has been explicitly set
	// by the config or by the user. It's required to guarantee backwards
	// compatibility with older versions of libpod for which we must query the
	// database configuration. Not included in the on-disk config.
	StorageConfigRunRootSet bool `toml:"-"`

	// StorageConfigGraphRootSet indicates if the RunRoot has been explicitly
	// set by the config or by the user. It's required to guarantee backwards
	// compatibility with older versions of libpod for which we must query the
	// database configuration. Not included in the on-disk config.
	StorageConfigGraphRootSet bool `toml:"-"`

	// StorageConfigGraphDriverNameSet indicates if the GraphDriverName has been
	// explicitly set by the config or by the user. It's required to guarantee
	// backwards compatibility with older versions of libpod for which we must
	// query the database configuration. Not included in the on-disk config.
	StorageConfigGraphDriverNameSet bool `toml:"-"`

	// StaticDirSet indicates if the StaticDir has been explicitly set by the
	// config or by the user. It's required to guarantee backwards compatibility
	// with older versions of libpod for which we must query the database
	// configuration. Not included in the on-disk config.
	StaticDirSet bool `toml:"-"`

	// VolumePathSet indicates if the VolumePath has been explicitly set by the
	// config or by the user. It's required to guarantee backwards compatibility
	// with older versions of libpod for which we must query the database
	// configuration. Not included in the on-disk config.
	VolumePathSet bool `toml:"-"`

	// TmpDirSet indicates if the TmpDir has been explicitly set by the config
	// or by the user. It's required to guarantee backwards compatibility with
	// older versions of libpod for which we must query the database
	// configuration. Not included in the on-disk config.
	TmpDirSet bool `toml:"-"`
}

// NetworkConfig represents the "network" TOML config table
type NetworkConfig struct {
	// CNIPluginDirs is where CNI plugin binaries are stored.
	CNIPluginDirs []string `toml:"cni_plugin_dirs,omitempty"`

	// DefaultNetwork is the network name of the default CNI network
	// to attach pods to.
	DefaultNetwork string `toml:"default_network,omitempty"`

	// DefaultSubnet is the subnet to be used for the default CNI network.
	// If a network with the name given in DefaultNetwork is not present
	// then a new network using this subnet will be created.
	// Must be a valid IPv4 CIDR block.
	DefaultSubnet string `toml:"default_subnet,omitempty"`

	// NetworkConfigDir is where CNI network configuration files are stored.
	NetworkConfigDir string `toml:"network_config_dir,omitempty"`
}

// SecretConfig represents the "secret" TOML config table
type SecretConfig struct {
	// Driver specifies the secret driver to use.
	// Current valid value:
	//  * file
	//  * pass
	Driver string `toml:"driver,omitempty"`
	// Opts contains driver specific options
	Opts map[string]string `toml:"opts,omitempty"`
}

// Destination represents destination for remote service
type Destination struct {
	// URI, required. Example: ssh://root@example.com:22/run/podman/podman.sock
	URI string `toml:"uri"`

	// Identity file with ssh key, optional
	Identity string `toml:"identity,omitempty"`
}

// NewConfig creates a new Config. It starts with an empty config and, if
// specified, merges the config at `userConfigPath` path.  Depending if we're
// running as root or rootless, we then merge the system configuration followed
// by merging the default config (hard-coded default in memory).
// Note that the OCI runtime is hard-set to `crun` if we're running on a system
// with cgroupv2v2. Other OCI runtimes are not yet supporting cgroupv2v2. This
// might change in the future.
func NewConfig(userConfigPath string) (*Config, error) {

	// Generate the default config for the system
	config, err := DefaultConfig()
	if err != nil {
		return nil, err
	}

	// Now, gather the system configs and merge them as needed.
	configs, err := systemConfigs()
	if err != nil {
		return nil, errors.Wrap(err, "finding config on system")
	}
	for _, path := range configs {
		// Merge changes in later configs with the previous configs.
		// Each config file that specified fields, will override the
		// previous fields.
		if err = readConfigFromFile(path, config); err != nil {
			return nil, errors.Wrapf(err, "reading system config %q", path)
		}
		logrus.Debugf("Merged system config %q", path)
		logrus.Tracef("%+v", config)
	}

	// If the caller specified a config path to use, then we read it to
	// override the system defaults.
	if userConfigPath != "" {
		var err error
		// readConfigFromFile reads in container config in the specified
		// file and then merge changes with the current default.
		if err = readConfigFromFile(userConfigPath, config); err != nil {
			return nil, errors.Wrapf(err, "reading user config %q", userConfigPath)
		}
		logrus.Debugf("Merged user config %q", userConfigPath)
		logrus.Tracef("%+v", config)
	}
	config.addCAPPrefix()

	if err := config.Validate(); err != nil {
		return nil, err
	}

	return config, nil
}

// readConfigFromFile reads the specified config file at `path` and attempts to
// unmarshal its content into a Config. The config param specifies the previous
// default config. If the path, only specifies a few fields in the Toml file
// the defaults from the config parameter will be used for all other fields.
func readConfigFromFile(path string, config *Config) error {
	logrus.Tracef("Reading configuration file %q", path)
	meta, err := toml.DecodeFile(path, config)
	if err != nil {
		return errors.Wrapf(err, "decode configuration %v", path)
	}
	keys := meta.Undecoded()
	if len(keys) > 0 {
		logrus.Warningf("Failed to decode the keys %q from %q.", keys, path)
	}

	return nil
}

// addConfigs will search one level in the config dirPath for config files
// If the dirPath does not exist, addConfigs will return nil
func addConfigs(dirPath string, configs []string) ([]string, error) {
	newConfigs := []string{}

	err := filepath.Walk(dirPath,
		// WalkFunc to read additional configs
		func(path string, info os.FileInfo, err error) error {
			switch {
			case err != nil:
				// return error (could be a permission problem)
				return err
			case info == nil:
				// this should only happen when err != nil but let's be sure
				return nil
			case info.IsDir():
				if path != dirPath {
					// make sure to not recurse into sub-directories
					return filepath.SkipDir
				}
				// ignore directories
				return nil
			default:
				// only add *.conf files
				if strings.HasSuffix(path, ".conf") {
					newConfigs = append(newConfigs, path)
				}
				return nil
			}
		},
	)
	if os.IsNotExist(err) {
		err = nil
	}
	sort.Strings(newConfigs)
	return append(configs, newConfigs...), err
}

// Returns the list of configuration files, if they exist in order of hierarchy.
// The files are read in order and each new file can/will override previous
// file settings.
func systemConfigs() ([]string, error) {
	var err error
	configs := []string{}
	path := os.Getenv("CONTAINERS_CONF")
	if path != "" {
		if _, err := os.Stat(path); err != nil {
			return nil, errors.Wrap(err, "CONTAINERS_CONF file")
		}
		return append(configs, path), nil
	}
	if _, err := os.Stat(DefaultContainersConfig); err == nil {
		configs = append(configs, DefaultContainersConfig)
	}
	if _, err := os.Stat(OverrideContainersConfig); err == nil {
		configs = append(configs, OverrideContainersConfig)
	}
	configs, err = addConfigs(OverrideContainersConfig+".d", configs)
	if err != nil {
		return nil, err
	}

	path, err = ifRootlessConfigPath()
	if err != nil {
		return nil, err
	}
	if path != "" {
		if _, err := os.Stat(path); err == nil {
			configs = append(configs, path)
		}
		configs, err = addConfigs(path+".d", configs)
		if err != nil {
			return nil, err
		}
	}
	return configs, nil
}

// CheckCgroupsAndAdjustConfig checks if we're running rootless with the systemd
// cgroup manager. In case the user session isn't available, we're switching the
// cgroup manager to cgroupfs.  Note, this only applies to rootless.
func (c *Config) CheckCgroupsAndAdjustConfig() {
	if !unshare.IsRootless() || c.Engine.CgroupManager != SystemdCgroupsManager {
		return
	}

	session := os.Getenv("DBUS_SESSION_BUS_ADDRESS")
	hasSession := session != ""
	if hasSession {
		for _, part := range strings.Split(session, ",") {
			if strings.HasPrefix(part, "unix:path=") {
				_, err := os.Stat(strings.TrimPrefix(part, "unix:path="))
				hasSession = err == nil
				break
			}
		}
	}

	if !hasSession && unshare.GetRootlessUID() != 0 {
		logrus.Warningf("The cgroupv2 manager is set to systemd but there is no systemd user session available")
		logrus.Warningf("For using systemd, you may need to login using an user session")
		logrus.Warningf("Alternatively, you can enable lingering with: `loginctl enable-linger %d` (possibly as root)", unshare.GetRootlessUID())
		logrus.Warningf("Falling back to --cgroup-manager=cgroupfs")
		c.Engine.CgroupManager = CgroupfsCgroupsManager
	}
}

func (c *Config) addCAPPrefix() {
	toCAPPrefixed := func(cap string) string {
		if !strings.HasPrefix(strings.ToLower(cap), "cap_") {
			return "CAP_" + strings.ToUpper(cap)
		}
		return cap
	}
	for i, cap := range c.Containers.DefaultCapabilities {
		c.Containers.DefaultCapabilities[i] = toCAPPrefixed(cap)
	}
}

// Validate is the main entry point for library configuration validation.
func (c *Config) Validate() error {

	if err := c.Containers.Validate(); err != nil {
		return errors.Wrap(err, "validating containers config")
	}

	if !c.Containers.EnableLabeling {
		selinux.SetDisabled()
	}

	if err := c.Engine.Validate(); err != nil {
		return errors.Wrap(err, "validating engine configs")
	}

	if err := c.Network.Validate(); err != nil {
		return errors.Wrap(err, "validating network configs")
	}

	return nil
}

func (c *EngineConfig) findRuntime() string {
	// Search for crun first followed by runc, kata, runsc
	for _, name := range []string{"crun", "runc", "kata", "runsc"} {
		for _, v := range c.OCIRuntimes[name] {
			if _, err := os.Stat(v); err == nil {
				return name
			}
		}
		if path, err := exec.LookPath(name); err == nil {
			logrus.Debugf("Found default OCI runtime %s path via PATH environment variable", path)
			return name
		}
	}
	return ""
}

// Validate is the main entry point for Engine configuration validation
// It returns an `error` on validation failure, otherwise
// `nil`.
func (c *EngineConfig) Validate() error {
	if err := c.validatePaths(); err != nil {
		return err
	}

	// Check if the pullPolicy from containers.conf is valid
	// if it is invalid returns the error
	pullPolicy := strings.ToLower(c.PullPolicy)
	if _, err := ValidatePullPolicy(pullPolicy); err != nil {
		return errors.Wrapf(err, "invalid pull type from containers.conf %q", c.PullPolicy)
	}
	return nil
}

// Validate is the main entry point for containers configuration validation
// It returns an `error` on validation failure, otherwise
// `nil`.
func (c *ContainersConfig) Validate() error {

	if err := c.validateUlimits(); err != nil {
		return err
	}

	if err := c.validateDevices(); err != nil {
		return err
	}

	if err := c.validateTZ(); err != nil {
		return err
	}

	if err := c.validateUmask(); err != nil {
		return err
	}

	if c.LogSizeMax >= 0 && c.LogSizeMax < OCIBufSize {
		return errors.Errorf("log size max should be negative or >= %d", OCIBufSize)
	}

	if _, err := units.FromHumanSize(c.ShmSize); err != nil {
		return errors.Errorf("invalid --shm-size %s, %q", c.ShmSize, err)
	}

	return nil
}

// Validate is the main entry point for network configuration validation.
// The parameter `onExecution` specifies if the validation should include
// execution checks. It returns an `error` on validation failure, otherwise
// `nil`.
func (c *NetworkConfig) Validate() error {
	expectedConfigDir := _cniConfigDir
	if unshare.IsRootless() {
		home, err := unshare.HomeDir()
		if err != nil {
			return err
		}
		expectedConfigDir = filepath.Join(home, _cniConfigDirRootless)
	}
	if c.NetworkConfigDir != expectedConfigDir {
		err := isDirectory(c.NetworkConfigDir)
		if err != nil && !os.IsNotExist(err) {
			return errors.Wrapf(err, "invalid network_config_dir: %s", c.NetworkConfigDir)
		}
	}

	if stringsEq(c.CNIPluginDirs, DefaultCNIPluginDirs) {
		return nil
	}

	for _, pluginDir := range c.CNIPluginDirs {
		if err := isDirectory(pluginDir); err == nil {
			return nil
		}
	}

	return errors.Errorf("invalid cni_plugin_dirs: %s", strings.Join(c.CNIPluginDirs, ","))
}

// FindConmon iterates over (*Config).ConmonPath and returns the path
// to first (version) matching conmon binary. If non is found, we try
// to do a path lookup of "conmon".
func (c *Config) FindConmon() (string, error) {
	foundOutdatedConmon := false
	for _, path := range c.Engine.ConmonPath {
		stat, err := os.Stat(path)
		if err != nil {
			continue
		}
		if stat.IsDir() {
			continue
		}
		if err := probeConmon(path); err != nil {
			logrus.Warnf("Conmon at %s invalid: %v", path, err)
			foundOutdatedConmon = true
			continue
		}
		logrus.Debugf("Using conmon: %q", path)
		return path, nil
	}

	// Search the $PATH as last fallback
	if path, err := exec.LookPath("conmon"); err == nil {
		if err := probeConmon(path); err != nil {
			logrus.Warnf("Conmon at %s is invalid: %v", path, err)
			foundOutdatedConmon = true
		} else {
			logrus.Debugf("Using conmon from $PATH: %q", path)
			return path, nil
		}
	}

	if foundOutdatedConmon {
		return "", errors.Wrapf(ErrConmonOutdated,
			"please update to v%d.%d.%d or later",
			_conmonMinMajorVersion, _conmonMinMinorVersion, _conmonMinPatchVersion)
	}

	return "", errors.Wrapf(ErrInvalidArg,
		"could not find a working conmon binary (configured options: %v)",
		c.Engine.ConmonPath)
}

// GetDefaultEnv returns the environment variables for the container.
// It will check the HTTPProxy and HostEnv booleans and add the appropriate
// environment variables to the container.
func (c *Config) GetDefaultEnv() []string {
	return c.GetDefaultEnvEx(c.Containers.EnvHost, c.Containers.HTTPProxy)
}

// GetDefaultEnvEx returns the environment variables for the container.
// It will check the HTTPProxy and HostEnv boolean parameters and return the appropriate
// environment variables for the container.
func (c *Config) GetDefaultEnvEx(envHost, httpProxy bool) []string {
	var env []string
	if envHost {
		env = append(env, os.Environ()...)
	} else if httpProxy {
		proxy := []string{"http_proxy", "https_proxy", "ftp_proxy", "no_proxy", "HTTP_PROXY", "HTTPS_PROXY", "FTP_PROXY", "NO_PROXY"}
		for _, p := range proxy {
			if val, ok := os.LookupEnv(p); ok {
				env = append(env, fmt.Sprintf("%s=%s", p, val))
			}
		}
	}
	return append(env, c.Containers.Env...)
}

// Capabilities returns the capabilities parses the Add and Drop capability
// list from the default capabiltiies for the container
func (c *Config) Capabilities(user string, addCapabilities, dropCapabilities []string) ([]string, error) {

	userNotRoot := func(user string) bool {
		if user == "" || user == "root" || user == "0" {
			return false
		}
		return true
	}

	defaultCapabilities := c.Containers.DefaultCapabilities
	if userNotRoot(user) {
		defaultCapabilities = []string{}
	}

	return capabilities.MergeCapabilities(defaultCapabilities, addCapabilities, dropCapabilities)
}

// Device parses device mapping string to a src, dest & permissions string
// Valid values for device looklike:
//    '/dev/sdc"
//    '/dev/sdc:/dev/xvdc"
//    '/dev/sdc:/dev/xvdc:rwm"
//    '/dev/sdc:rm"
func Device(device string) (src, dst, permissions string, err error) {
	permissions = "rwm"
	split := strings.Split(device, ":")
	switch len(split) {
	case 3:
		if !IsValidDeviceMode(split[2]) {
			return "", "", "", errors.Errorf("invalid device mode: %s", split[2])
		}
		permissions = split[2]
		fallthrough
	case 2:
		if IsValidDeviceMode(split[1]) {
			permissions = split[1]
		} else {
			if split[1] == "" || split[1][0] != '/' {
				return "", "", "", errors.Errorf("invalid device mode: %s", split[1])
			}
			dst = split[1]
		}
		fallthrough
	case 1:
		if !strings.HasPrefix(split[0], "/dev/") {
			return "", "", "", errors.Errorf("invalid device mode: %s", split[0])
		}
		src = split[0]
	default:
		return "", "", "", errors.Errorf("invalid device specification: %s", device)
	}

	if dst == "" {
		dst = src
	}
	return src, dst, permissions, nil
}

// IsValidDeviceMode checks if the mode for device is valid or not.
// IsValid mode is a composition of r (read), w (write), and m (mknod).
func IsValidDeviceMode(mode string) bool {
	var legalDeviceMode = map[rune]bool{
		'r': true,
		'w': true,
		'm': true,
	}
	if mode == "" {
		return false
	}
	for _, c := range mode {
		if !legalDeviceMode[c] {
			return false
		}
		legalDeviceMode[c] = false
	}
	return true
}

// resolveHomeDir converts a path referencing the home directory via "~"
// to an absolute path
func resolveHomeDir(path string) (string, error) {
	// check if the path references the home dir to avoid work
	// don't use strings.HasPrefix(path, "~") as this doesn't match "~" alone
	// use strings.HasPrefix(...) to not match "something/~/something"
	if !(path == "~" || strings.HasPrefix(path, "~/")) {
		// path does not reference home dir -> Nothing to do
		return path, nil
	}

	// only get HomeDir when necessary
	home, err := unshare.HomeDir()
	if err != nil {
		return "", err
	}

	// replace the first "~" (start of path) with the HomeDir to resolve "~"
	return strings.Replace(path, "~", home, 1), nil
}

func rootlessConfigPath() (string, error) {
	if configHome := os.Getenv("XDG_CONFIG_HOME"); configHome != "" {
		return filepath.Join(configHome, _configPath), nil
	}
	home, err := unshare.HomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, UserOverrideContainersConfig), nil
}

func stringsEq(a, b []string) bool {

	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

var (
	configErr   error
	configMutex sync.Mutex
	config      *Config
)

// Default returns the default container config.
// Configuration files will be read in the following files:
// * /usr/share/containers/containers.conf
// * /etc/containers/containers.conf
// * $HOME/.config/containers/containers.conf # When run in rootless mode
// Fields in latter files override defaults set in previous files and the
// default config.
// None of these files are required, and not all fields need to be specified
// in each file, only the fields you want to override.
// The system defaults container config files can be overwritten using the
// CONTAINERS_CONF environment variable.  This is usually done for testing.
func Default() (*Config, error) {
	configMutex.Lock()
	defer configMutex.Unlock()
	if config != nil || configErr != nil {
		return config, configErr
	}
	return defConfig()
}

func defConfig() (*Config, error) {
	config, configErr = NewConfig("")
	return config, configErr
}

func Path() string {
	if path := os.Getenv("CONTAINERS_CONF"); path != "" {
		return path
	}
	if unshare.IsRootless() {
		if rpath, err := rootlessConfigPath(); err == nil {
			return rpath
		}
		return "$HOME/" + UserOverrideContainersConfig
	}
	return OverrideContainersConfig
}

// ReadCustomConfig reads the custom config and only generates a config based on it
// If the custom config file does not exists, function will return an empty config
func ReadCustomConfig() (*Config, error) {
	path, err := customConfigFile()
	if err != nil {
		return nil, err
	}
	// hack since Ommitempty does not seem to work with Write
	c, err := Default()
	if err != nil {
		if os.IsNotExist(errors.Cause(err)) {
			c, err = DefaultConfig()
		}
		if err != nil {
			return nil, err
		}
	}

	newConfig := &Config{}
	if _, err := os.Stat(path); err == nil {
		if err := readConfigFromFile(path, newConfig); err != nil {
			return nil, err
		}
	} else {
		if !os.IsNotExist(err) {
			return nil, err
		}
	}
	newConfig.Containers.LogSizeMax = c.Containers.LogSizeMax
	newConfig.Containers.PidsLimit = c.Containers.PidsLimit
	newConfig.Containers.UserNSSize = c.Containers.UserNSSize
	newConfig.Engine.NumLocks = c.Engine.NumLocks
	newConfig.Engine.StopTimeout = c.Engine.StopTimeout
	return newConfig, nil
}

// Write writes the configuration to the default file
func (c *Config) Write() error {
	var err error
	path, err := customConfigFile()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	configFile, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer configFile.Close()
	enc := toml.NewEncoder(configFile)
	if err := enc.Encode(c); err != nil {
		return err
	}
	return nil
}

// Reload clean the cached config and reloads the configuration from containers.conf files
// This function is meant to be used for long-running processes that need to reload potential changes made to
// the cached containers.conf files.
func Reload() (*Config, error) {
	configMutex.Lock()
	defer configMutex.Unlock()
	return defConfig()
}

func (c *Config) ActiveDestination() (uri, identity string, err error) {
	if uri, found := os.LookupEnv("CONTAINER_HOST"); found {
		if v, found := os.LookupEnv("CONTAINER_SSHKEY"); found {
			identity = v
		}
		return uri, identity, nil
	}
	connEnv := os.Getenv("CONTAINER_CONNECTION")
	switch {
	case connEnv != "":
		d, found := c.Engine.ServiceDestinations[connEnv]
		if !found {
			return "", "", errors.Errorf("environment variable CONTAINER_CONNECTION=%q service destination not found", connEnv)
		}
		return d.URI, d.Identity, nil

	case c.Engine.ActiveService != "":
		d, found := c.Engine.ServiceDestinations[c.Engine.ActiveService]
		if !found {
			return "", "", errors.Errorf("%q service destination not found", c.Engine.ActiveService)
		}
		return d.URI, d.Identity, nil
	case c.Engine.RemoteURI != "":
		return c.Engine.RemoteURI, c.Engine.RemoteIdentity, nil
	}
	return "", "", errors.New("no service destination configured")
}

// FindHelperBinary will search the given binary name in the configured directories.
// If searchPATH is set to true it will also search in $PATH.
func (c *Config) FindHelperBinary(name string, searchPATH bool) (string, error) {
	for _, path := range c.Engine.HelperBinariesDir {
		fullpath := filepath.Join(path, name)
		if fi, err := os.Stat(fullpath); err == nil && fi.Mode().IsRegular() {
			return fullpath, nil
		}
	}
	if searchPATH {
		return exec.LookPath(name)
	}
	if len(c.Engine.HelperBinariesDir) == 0 {
		return "", errors.Errorf("could not find %q because there are no helper binary directories configured", name)
	}
	return "", errors.Errorf("could not find %q in one of %v", name, c.Engine.HelperBinariesDir)
}
