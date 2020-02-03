package config

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/BurntSushi/toml"
	"github.com/containers/common/pkg/caps"
	"github.com/containers/common/pkg/unshare"
	"github.com/containers/storage"
	units "github.com/docker/go-units"
	selinux "github.com/opencontainers/selinux/go-selinux"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	// DefaultContainersConfig holds the default containers config path
	DefaultContainersConfig = "/usr/share/containers/containers.conf"
	// OverrideContainersConfig holds the default config paths overridden by the root user
	OverrideContainersConfig = "/etc/containers/containers.conf"
	// UserOverrideContainersConfig holds the containers config path overridden by the rootless user
	UserOverrideContainersConfig = ".config/containers/containers.conf"
)

// RuntimeStateStore is a constant indicating which state store implementation
// should be used by libpod
type RuntimeStateStore int

const (
	// InvalidStateStore is an invalid state store
	InvalidStateStore RuntimeStateStore = iota
	// InMemoryStateStore is an in-memory state that will not persist data
	// on containers and pods between libpod instances or after system
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
	// Libpod specifies how the container engine based on Libpod will run
	Libpod LibpodConfig `toml:"libpod"`
	// Network section defines the configuration of CNI Plugins
	Network NetworkConfig `toml:"network"`
}

// ContainersConfig represents the "containers" TOML config table
// containers global options for containers tools
type ContainersConfig struct {

	// Devices to add to all containers
	AdditionalDevices []string `toml:"additional_devices"`

	// Volumes to add to all containers
	AdditionalVolumes []string `toml:"additional_volumes"`

	// ApparmorProfile is the apparmor profile name which is used as the
	// default for the runtime.
	ApparmorProfile string `toml:"apparmor_profile"`

	// CGroupManager is the CGroup Manager to use Valid values are "cgroupfs"
	// and "systemd".
	CgroupManager string `toml:"cgroup_manager"`

	// Default way to create a cgroup namespace for the container
	CgroupNS string `toml:"cgroupns"`

	// Capabilities to add to all containers.
	DefaultCapabilities []string `toml:"default_capabilities"`

	// Sysctls to add to all containers.
	DefaultSysctls []string `toml:"default_sysctls"`

	// DefaultUlimits specifies the default ulimits to apply to containers
	DefaultUlimits []string `toml:"default_ulimits"`

	// DefaultMountsFile is the path to the default mounts file for testing
	DefaultMountsFile string `toml:"-"`

	// DNSServers set default DNS servers.
	DNSServers []string `toml:"dns_servers"`

	// DNSOptions set default DNS options.
	DNSOptions []string `toml:"dns_options"`

	// DNSSearches set default DNS search domains.
	DNSSearches []string `toml:"dns_searches"`

	// EnableLabeling tells the container engines whether to use MAC
	// Labeling to separate containers (SELinux)
	EnableLabeling bool `toml:"label"`

	// Env is the environment variable list for container process.
	Env []string `toml:"env"`

	// EnvHost Pass all host environment variables into the container.
	EnvHost bool `toml:"env_host"`

	// HTTPProxy is the proxy environment variable list to apply to container process
	HTTPProxy bool `toml:"http_proxy"`

	// Init tells container runtimes whether to run init inside the
	// container that forwards signals and reaps processes.
	Init bool `toml:"init"`

	// InitPath is the path for init to run if the Init bool is enabled
	InitPath string `toml:"init_path"`

	// IPCNS way to to create a ipc namespace for the container
	IPCNS string `toml:"ipcns"`

	// LogDriver  for the container.  For example: k8s-file and journald
	LogDriver string `toml:"log_driver"`

	// LogSizeMax is the maximum number of bytes after which the log file
	// will be truncated. It can be expressed as a human-friendly string
	// that is parsed to bytes.
	// Negative values indicate that the log file won't be truncated.
	LogSizeMax int64 `toml:"log_size_max"`

	// NetNS indicates how to create a network namespace for the container
	NetNS string `toml:"netns"`

	// NoHosts tells container engine whether to create its own /etc/hosts
	NoHosts bool `toml:"no_hosts"`

	// PidsLimit is the number of processes each container is restricted to
	// by the cgroup process number controller.
	PidsLimit int64 `toml:"pids_limit"`

	// PidNS indicates how to create a pid namespace for the container
	PidNS string `toml:"pidns"`

	// SeccompProfile is the seccomp.json profile path which is used as the
	// default for the runtime.
	SeccompProfile string `toml:"seccomp_profile"`

	// ShmSize holds the size of /dev/shm.
	ShmSize string `toml:"shm_size"`

	// SignaturePolicyPath is the path to a signature policy to use for
	// validating images. If left empty, the containers/image default signature
	// policy will be used.
	SignaturePolicyPath string `toml:"_"`

	// UTSNS indicates how to create a UTS namespace for the container
	UTSNS string `toml:"utsns"`

	// UserNS indicates how to create a User namespace for the container
	UserNS string `toml:"userns"`

	// UserNSSize how many UIDs to allocate for automatically created UserNS
	UserNSSize int `toml:"userns_size"`
}

// LibpodConfig contains configuration options used to set up a libpod runtime
type LibpodConfig struct {
	// NOTE: when changing this struct, make sure to update (*Config).Merge().

	// ConmonEnvVars are environment variables to pass to the Conmon binary
	// when it is launched.
	ConmonEnvVars []string `toml:"conmon_env_vars"`

	// ConmonPath is the path to the Conmon binary used for managing containers.
	// The first path pointing to a valid file will be used.
	ConmonPath []string `toml:"conmon_path"`

	//DetachKeys is the sequence of keys used to detach a container.
	DetachKeys string `toml:"detach_keys"`

	// EnablePortReservation determines whether libpod will reserve ports on the
	// host when they are forwarded to containers. When enabled, when ports are
	// forwarded to containers, they are held open by conmon as long as the
	// container is running, ensuring that they cannot be reused by other
	// programs on the host. However, this can cause significant memory usage if
	// a container has many ports forwarded to it. Disabling this can save
	// memory.
	EnablePortReservation bool `toml:"enable_port_reservation"`

	// EventsLogFilePath is where the events log is stored.
	EventsLogFilePath string `toml:"events_logfile_path"`

	// EventsLogger determines where events should be logged.
	EventsLogger string `toml:"events_logger"`

	// configuration files. When the same filename is present in in
	// multiple directories, the file in the directory listed last in
	// this slice takes precedence.
	HooksDir []string `toml:"hooks_dir"`

	// ImageDefaultTransport is the default transport method used to fetch
	// images.
	ImageDefaultTransport string `toml:"image_default_transport"`

	// InfraCommand is the command run to start up a pod infra container.
	InfraCommand string `toml:"infra_command"`

	// InfraImage is the image a pod infra container will use to manage
	// namespaces.
	InfraImage string `toml:"infra_image"`

	// InitPath is the path to the container-init binary.
	InitPath string `toml:"init_path"`

	// LockType is the type of locking to use.
	LockType string `toml:"lock_type,omitempty"`

	// Namespace is the libpod namespace to use. Namespaces are used to create
	// scopes to separate containers and pods in the state. When namespace is
	// set, libpod will only view containers and pods in the same namespace. All
	// containers and pods created will default to the namespace set here. A
	// namespace of "", the empty string, is equivalent to no namespace, and all
	// containers and pods will be visible. The default namespace is "".
	Namespace string `toml:"namespace,omitempty"`

	// NetworkCmdPath is the path to the slirp4netns binary.
	NetworkCmdPath string `toml:"network_cmd_path"`

	// NoPivotRoot sets whether to set no-pivot-root in the OCI runtime.
	NoPivotRoot bool `toml:"no_pivot_root"`

	// NumLocks is the number of locks to make available for containers and
	// pods.
	NumLocks uint32 `toml:"num_locks,omitempty"`

	// OCIRuntime is the OCI runtime to use.
	OCIRuntime string `toml:"runtime"`

	// OCIRuntimes are the set of configured OCI runtimes (default is runc).
	OCIRuntimes map[string][]string `toml:"runtimes"`

	// RuntimeSupportsJSON is the list of the OCI runtimes that support
	// --format=json.
	RuntimeSupportsJSON []string `toml:"runtime_supports_json"`

	// RuntimeSupportsNoCgroups is a list of OCI runtimes that support
	// running containers without CGroups.
	RuntimeSupportsNoCgroups []string `toml:"runtime_supports_nocgroups"`

	// SetOptions contains a subset of config options. It's used to indicate if
	// a given option has either been set by the user or by a parsed libpod
	// configuration file.  If not, the corresponding option might be
	// overwritten by values from the database.  This behavior guarantess
	// backwards compat with older version of libpod and Podman.
	SetOptions

	// SDNotify tells container engine to allow containers to notify the host systemd of
	// readiness using the SD_NOTIFY mechanism.
	SDNotify bool

	// StateType is the type of the backing state store. Avoid using multiple
	// values for this with the same containers/storage configuration on the
	// same system. Different state types do not interact, and each will see a
	// separate set of containers, which may cause conflicts in
	// containers/storage. As such this is not exposed via the config file.
	StateType RuntimeStateStore `toml:"-"`

	// StaticDir is the path to a persistent directory to store container
	// files.
	StaticDir string `toml:"static_dir"`

	// StorageConfig is the configuration used by containers/storage Not
	// included in the on-disk config, use the dedicated containers/storage
	// configuration file instead.
	StorageConfig storage.StoreOptions `toml:"-"`

	// TmpDir is the path to a temporary directory to store per-boot container
	// files. Must be stored in a tmpfs.
	TmpDir string `toml:"tmp_dir"`

	// VolumePath is the default location that named volumes will be created
	// under. This convention is followed by the default volume driver, but
	// may not be by other drivers.
	VolumePath string `toml:"volume_path"`
}

// SetOptions contains a subset of options in a Config. It's used to indicate if
// a given option has either been set by the user or by a parsed libpod
// configuration file.  If not, the corresponding option might be overwritten by
// values from the database.  This behavior guarantess backwards compat with
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
	CNIPluginDirs []string `toml:"cni_plugin_dirs"`

	// DefaultNetwork is the network name of the default CNI network
	// to attach pods to.
	DefaultNetwork string `toml:"default_network,omitempty"`

	// NetworkConfigDir is where CNI network configuration files are stored.
	NetworkConfigDir string `toml:"network_config_dir"`
}

// NewConfig creates a new Config. It starts with an empty config and, if
// specified, merges the config at `userConfigPath` path.  Depending if we're
// running as root or rootless, we then merge the system configuration followed
// by merging the default config (hard-coded default in memory).
// Note that the OCI runtime is hard-set to `crun` if we're running on a system
// with cgroupsv2. Other OCI runtimes are not yet supporting cgroupsv2. This
// might change in the future.
func NewConfig(userConfigPath string) (*Config, error) {

	// Generate the default config for the system
	config, err := DefaultConfig()
	if err != nil {
		return nil, err
	}

	// If the caller specified a config path to use, then we read this
	// rather then using the system defaults.
	if userConfigPath != "" {
		var err error
		// ReadConfigFromFile reads in container config in the specified
		// file and then merge changes with the current default.
		config, err = ReadConfigFromFile(userConfigPath, config)
		if err != nil {
			return nil, errors.Wrapf(err, "error reading user config %q", userConfigPath)
		}
	}

	// Now, gather the system configs and merge them as needed.
	configs, err := systemConfigs()
	if err != nil {
		return nil, errors.Wrapf(err, "error finding config on system")
	}
	for _, path := range configs {
		// Merge changes in later configs with the previous configs.
		// Each config file that specified fields, will override the
		// previous fields.
		config, err := ReadConfigFromFile(path, config)
		if err != nil {
			return nil, errors.Wrapf(err, "error reading system config %q", path)
		}
		logrus.Debugf("Merged system config %q: %v", path, config)
	}

	config.checkCgroupsAndAdjustConfig()
	config.addCAPPrefix()

	if err := config.Validate(true); err != nil {
		return nil, err
	}

	return config, nil
}

// ReadConfigFromFile reads the specified config file at `path` and attempts to
// unmarshal its content into a Config. The config param specifies the previous
// default config.  If the path, only specifies a few fields in the Toml file
// the defaults from the config parameter will be used for all other fields.
func ReadConfigFromFile(path string, config *Config) (*Config, error) {
	logrus.Debugf("Reading configuration file %q", path)
	_, err := toml.DecodeFile(path, config)
	if err != nil {
		return nil, fmt.Errorf("unable to decode configuration %v: %v", path, err)
	}
	if config.Libpod.VolumePath != "" {
		config.Libpod.VolumePathSet = true
	}
	if config.Libpod.StaticDir != "" {
		config.Libpod.StaticDirSet = true
	}
	if config.Libpod.TmpDir != "" {
		config.Libpod.TmpDirSet = true
	}

	return config, err
}

// Returns the list of configuration files, if they exist in order of hierarchy.
// The files are read in order and each new file can/will override previous
// file settings.
func systemConfigs() ([]string, error) {
	configs := []string{}
	path := os.Getenv("CONTAINERS_CONF")
	if path != "" {
		if _, err := os.Stat(path); err != nil {
			return nil, errors.Wrap(err, "failed to stat of %s from CONTAINERS_CONF environment variable")
		}
		return append(configs, path), nil
	}
	if _, err := os.Stat(DefaultContainersConfig); err == nil {
		configs = append(configs, DefaultContainersConfig)
	}
	if _, err := os.Stat(OverrideContainersConfig); err == nil {
		configs = append(configs, OverrideContainersConfig)
	}
	if unshare.IsRootless() {
		path, err := rootlessConfigPath()
		if err != nil {
			return nil, err
		}
		if _, err := os.Stat(path); err == nil {
			configs = append(configs, path)
		}
	}
	return configs, nil
}

// checkCgroupsAndAdjustConfig checks if we're running rootless with the systemd
// cgroup manager. In case the user session isn't available, we're switching the
// cgroup manager to cgroupfs.  Note, this only applies to rootless.
func (c *Config) checkCgroupsAndAdjustConfig() {
	if !unshare.IsRootless() || c.Containers.CgroupManager != SystemdCgroupsManager {
		return
	}

	session := os.Getenv("DBUS_SESSION_BUS_ADDRESS")
	hasSession := session != ""
	if hasSession && strings.HasPrefix(session, "unix:path=") {
		_, err := os.Stat(strings.TrimPrefix(session, "unix:path="))
		hasSession = err == nil
	}

	if !hasSession {
		logrus.Warningf("The cgroups manager is set to systemd but there is no systemd user session available")
		logrus.Warningf("For using systemd, you may need to login using an user session")
		logrus.Warningf("Alternatively, you can enable lingering with: `loginctl enable-linger %d` (possibly as root)", unshare.GetRootlessUID())
		logrus.Warningf("Falling back to --cgroup-manager=cgroupfs")
		c.Containers.CgroupManager = CgroupfsCgroupsManager
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
// The parameter `onExecution` specifies if the validation should include
// execution checks. It returns an `error` on validation failure, otherwise
// `nil`.
func (c *Config) Validate(onExecution bool) error {

	if err := c.Containers.Validate(); err != nil {
		return errors.Wrapf(err, "containers config")
	}

	if !unshare.IsRootless() {
		if err := c.Network.Validate(onExecution); err != nil {
			return errors.Wrapf(err, "network config")
		}
	}
	if !c.Containers.EnableLabeling {
		selinux.SetDisabled()
	}

	return nil
}

// Validate is the main entry point for Libpod configuration validation
// It returns an `error` on validation failure, otherwise
// `nil`.
func (c *LibpodConfig) Validate() error {
	// Relative paths can cause nasty bugs, because core paths we use could
	// shift between runs (or even parts of the program - the OCI runtime
	// uses a different working directory than we do, for example.
	if !filepath.IsAbs(c.StaticDir) {
		return fmt.Errorf("static directory must be an absolute path - instead got %q", c.StaticDir)
	}
	if !filepath.IsAbs(c.TmpDir) {
		return fmt.Errorf("temporary directory must be an absolute path - instead got %q", c.TmpDir)
	}
	if !filepath.IsAbs(c.VolumePath) {
		return fmt.Errorf("volume path must be an absolute path - instead got %q", c.VolumePath)
	}
	return nil
}

// Validate is the main entry point for containers configuration validation
// It returns an `error` on validation failure, otherwise
// `nil`.
func (c *ContainersConfig) Validate() error {
	for _, u := range c.DefaultUlimits {
		ul, err := units.ParseUlimit(u)
		if err != nil {
			return fmt.Errorf("unrecognized ulimit %s: %v", u, err)
		}
		_, err = ul.GetRlimit()
		if err != nil {
			return err
		}
	}

	for _, d := range c.AdditionalDevices {
		_, _, _, err := Device(d)
		if err != nil {
			return err
		}
	}

	if c.LogSizeMax >= 0 && c.LogSizeMax < OCIBufSize {
		return fmt.Errorf("log size max should be negative or >= %d", OCIBufSize)
	}

	if _, err := units.FromHumanSize(c.ShmSize); err != nil {
		return fmt.Errorf("invalid --shm-size %s, %q", c.ShmSize, err)
	}

	return nil
}

// Validate is the main entry point for network configuration validation.
// The parameter `onExecution` specifies if the validation should include
// execution checks. It returns an `error` on validation failure, otherwise
// `nil`.
func (c *NetworkConfig) Validate(onExecution bool) error {
	if onExecution {
		err := IsDirectory(c.NetworkConfigDir)
		if err != nil {
			if os.IsNotExist(err) {
				if err = os.MkdirAll(c.NetworkConfigDir, 0755); err != nil {
					return errors.Wrapf(err, "Cannot create network_config_dir: %s", c.NetworkConfigDir)
				}
			} else {
				return errors.Wrapf(err, "invalid network_config_dir: %s", c.NetworkConfigDir)
			}
		}

		for _, pluginDir := range c.CNIPluginDirs {
			if err := os.MkdirAll(pluginDir, 0755); err != nil {
				return errors.Wrapf(err, "invalid cni_plugin_dirs entry")
			}
		}
	}

	return nil
}

// DBConfig is a set of Libpod runtime configuration settings that are saved in
// a State when it is first created, and can subsequently be retrieved.
type DBConfig struct {
	LibpodRoot  string
	LibpodTmp   string
	StorageRoot string
	StorageTmp  string
	GraphDriver string
	VolumePath  string
}

// MergeDBConfig merges the configuration from the database.
func (c *Config) MergeDBConfig(dbConfig *DBConfig) error {

	if !c.Libpod.StorageConfigRunRootSet && dbConfig.StorageTmp != "" {
		if c.Libpod.StorageConfig.RunRoot != dbConfig.StorageTmp &&
			c.Libpod.StorageConfig.RunRoot != "" {
			logrus.Debugf("Overriding run root %q with %q from database",
				c.Libpod.StorageConfig.RunRoot, dbConfig.StorageTmp)
		}
		c.Libpod.StorageConfig.RunRoot = dbConfig.StorageTmp
	}

	if !c.Libpod.StorageConfigGraphRootSet && dbConfig.StorageRoot != "" {
		if c.Libpod.StorageConfig.GraphRoot != dbConfig.StorageRoot &&
			c.Libpod.StorageConfig.GraphRoot != "" {
			logrus.Debugf("Overriding graph root %q with %q from database",
				c.Libpod.StorageConfig.GraphRoot, dbConfig.StorageRoot)
		}
		c.Libpod.StorageConfig.GraphRoot = dbConfig.StorageRoot
	}

	if !c.Libpod.StorageConfigGraphDriverNameSet && dbConfig.GraphDriver != "" {
		if c.Libpod.StorageConfig.GraphDriverName != dbConfig.GraphDriver &&
			c.Libpod.StorageConfig.GraphDriverName != "" {
			logrus.Errorf("User-selected graph driver %q overwritten by graph driver %q from database - delete libpod local files to resolve",
				c.Libpod.StorageConfig.GraphDriverName, dbConfig.GraphDriver)
		}
		c.Libpod.StorageConfig.GraphDriverName = dbConfig.GraphDriver
	}

	if !c.Libpod.StaticDirSet && dbConfig.LibpodRoot != "" {
		if c.Libpod.StaticDir != dbConfig.LibpodRoot && c.Libpod.StaticDir != "" {
			logrus.Debugf("Overriding static dir %q with %q from database", c.Libpod.StaticDir, dbConfig.LibpodRoot)
		}
		c.Libpod.StaticDir = dbConfig.LibpodRoot
	}

	if !c.Libpod.TmpDirSet && dbConfig.LibpodTmp != "" {
		if c.Libpod.TmpDir != dbConfig.LibpodTmp && c.Libpod.TmpDir != "" {
			logrus.Debugf("Overriding tmp dir %q with %q from database", c.Libpod.TmpDir, dbConfig.LibpodTmp)
		}
		c.Libpod.TmpDir = dbConfig.LibpodTmp
		c.Libpod.EventsLogFilePath = filepath.Join(dbConfig.LibpodTmp, "events", "events.log")
	}

	if !c.Libpod.VolumePathSet && dbConfig.VolumePath != "" {
		if c.Libpod.VolumePath != dbConfig.VolumePath && c.Libpod.VolumePath != "" {
			logrus.Debugf("Overriding volume path %q with %q from database", c.Libpod.VolumePath, dbConfig.VolumePath)
		}
		c.Libpod.VolumePath = dbConfig.VolumePath
	}
	return nil
}

// FindConmon iterates over (*Config).ConmonPath and returns the path to first
// (version) matching conmon binary. If non is found, we try to do a path lookup
// of "conmon".
func (c *Config) FindConmon() (string, error) {
	foundOutdatedConmon := false
	for _, path := range c.Libpod.ConmonPath {
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
		c.Libpod.ConmonPath)
}

// GetDefaultEnv returns the environment variables for the container.
// It will checn the HTTPProxy and HostEnv booleans and add the appropriate
// environment variables to the container.
func (c *Config) GetDefaultEnv() []string {
	var env []string
	if c.Containers.EnvHost {
		env = append(env, os.Environ()...)
	} else if c.Containers.HTTPProxy {
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
func (c *Config) Capabilities(user string, addCapabilities, dropCapabilities []string) []string {

	userNotRoot := func(user string) bool {
		if user == "" || user == "root" || user == "0" {
			return false
		}
		return true
	}

	var capabilities []string
	defaultCapabilities := c.Containers.DefaultCapabilities
	if userNotRoot(user) {
		defaultCapabilities = []string{}
	}

	mapCap := make(map[string]bool, len(defaultCapabilities))
	for _, c := range addCapabilities {
		if strings.ToLower(c) == "all" {
			defaultCapabilities = caps.GetAllCapabilities()
			addCapabilities = nil
			break
		}
	}

	for _, c := range append(defaultCapabilities, addCapabilities...) {
		mapCap[c] = true
	}
	for _, c := range dropCapabilities {
		if "all" == strings.ToLower(c) {
			return capabilities
		}
		mapCap[c] = false
	}
	for cap, add := range mapCap {
		if add {
			capabilities = append(capabilities, cap)
		}
	}
	return capabilities
}

// Device parses device mapping string to a src, dest & permissions string
// Valid values for device looklike:
//    '/dev/sdc"
//    '/dev/sdc:/dev/xvdc"
//    '/dev/sdc:/dev/xvdc:rwm"
//    '/dev/sdc:rm"
func Device(device string) (string, string, string, error) {
	src := ""
	dst := ""
	permissions := "rwm"
	split := strings.Split(device, ":")
	switch len(split) {
	case 3:
		if !IsValidDeviceMode(split[2]) {
			return "", "", "", fmt.Errorf("invalid device mode: %s", split[2])
		}
		permissions = split[2]
		fallthrough
	case 2:
		if IsValidDeviceMode(split[1]) {
			permissions = split[1]
		} else {
			if len(split[1]) == 0 || split[1][0] != '/' {
				return "", "", "", fmt.Errorf("invalid device mode: %s", split[1])
			}
			dst = split[1]
		}
		fallthrough
	case 1:
		if !strings.HasPrefix(split[0], "/dev/") {
			return "", "", "", fmt.Errorf("invalid device mode: %s", split[0])
		}
		src = split[0]
	default:
		return "", "", "", fmt.Errorf("invalid device specification: %s", device)
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

// IsDirectory tests whether the given path exists and is a directory. It
// follows symlinks.
func IsDirectory(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	if !info.Mode().IsDir() {
		// Return a PathError to be consistent with os.Stat().
		return &os.PathError{
			Op:   "stat",
			Path: path,
			Err:  syscall.ENOTDIR,
		}
	}

	return nil
}

func rootlessConfigPath() (string, error) {
	home, err := unshare.HomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, UserOverrideContainersConfig), nil
}
