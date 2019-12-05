package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/BurntSushi/toml"
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

// tomlConfig is another way of looking at a Config, which is
// TOML-friendly (it has all of the explicit tables). It's just used for
// conversions.
type tomlConfig struct {
	Containers struct{ ContainersConfig } `toml:"containers"`
	Libpod     struct{ LibpodConfig }     `toml:"libpod"`
	Network    struct{ NetworkConfig }    `toml:"network"`
}

// Config contains configuration options for container tools
type Config struct {
	ContainersConfig
	LibpodConfig
	NetworkConfig
}

// ContainersConfig represents the "containers" TOML config table
// containers global options for containers tools
type ContainersConfig struct {

	// Devices to add to containers
	AdditionalDevices []string `toml:"additional_devices"`

	// ApparmorProfile is the apparmor profile name which is used as the
	// default for the runtime.
	ApparmorProfile string `toml:"apparmor_profile"`

	// CGroupManager is the CGroup Manager to use Valid values are "cgroupfs"
	// and "systemd".
	CgroupManager string `toml:"cgroup_manager"`

	// Capabilities to add to all containers.
	DefaultCapabilities []string `toml:"default_capabilities"`

	// DefaultMountsFile is the path to the default mounts file for testing
	// purposes only.
	DefaultMountsFile string `toml:"-"`

	// Sysctls to add to all containers.
	DefaultSysctls []string `toml:"default_sysctls"`

	// DefaultUlimits specifies the default ulimits to apply to containers
	DefaultUlimits []string `toml:"default_ulimits"`

	// EnableLabeling indicates whether the container tool will support container labeling.
	EnableLabeling bool `toml:"label"`

	// Env is the environment variable list for container process.
	Env []string `toml:"env"`

	// HooksDir holds paths to the directories containing hooks
	// configuration files. When the same filename is present in in
	// multiple directories, the file in the directory listed last in
	// this slice takes precedence.
	HooksDir []string `toml:"hooks_dir"`

	// Run an init inside the container that forwards signals and reaps processes.
	Init bool `toml:"init"`

	// HTTPProxy is the proxy environment variable list to apply to container process
	HTTPProxy []string `toml:"http_proxy"`

	// LogSizeMax is the maximum number of bytes after which the log file
	// will be truncated. It can be expressed as a human-friendly string
	// that is parsed to bytes.
	// Negative values indicate that the log file won't be truncated.
	LogSizeMax int64 `toml:"log_size_max"`

	// PidsLimit is the number of processes each container is restricted to
	// by the cgroup process number controller.
	PidsLimit int64 `toml:"pids_limit"`

	// SeccompProfile is the seccomp.json profile path which is used as the
	// default for the runtime.
	SeccompProfile string `toml:"seccomp_profile"`

	// ShmSize holds the size of /dev/shm.
	ShmSize string `toml:"shm_size"`

	// SignaturePolicyPath is the path to a signature policy to use for
	// validating images. If left empty, the containers/image default signature
	// policy will be used.
	SignaturePolicyPath string `toml:"signature_policy_path,omitempty"`
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

	// RuntimePath is the path to OCI runtime binary for launching containers.
	// The first path pointing to a valid file will be used This is used only
	// when there are no OCIRuntime/OCIRuntimes defined.  It is used only to be
	// backward compatible with older versions of Podman.
	RuntimePath []string `toml:"runtime_path"`

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

	// SDNotify tells Libpod to allow containers to notify the host systemd of
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

// DefaultCapabilities for the default_capabilities option in the containers.conf file
var DefaultCapabilities = []string{
	"CAP_AUDIT_WRITE",
	"CAP_CHOWN",
	"CAP_DAC_OVERRIDE",
	"CAP_FOWNER",
	"CAP_FSETID",
	"CAP_KILL",
	"CAP_MKNOD",
	"CAP_NET_BIND_SERVICE",
	"CAP_NET_RAW",
	"CAP_SETGID",
	"CAP_SETPCAP",
	"CAP_SETUID",
	"CAP_SYS_CHROOT",
}

// DefaultHooksDirs defines the default hooks directory
var DefaultHooksDirs = []string{"/usr/share/containers/oci/hooks.d"}

// NewConfig creates a new Config. It starts with an empty config and, if
// specified, merges the config at `userConfigPath` path.  Depending if we're
// running as root or rootless, we then merge the system configuration followed
// by merging the default config (hard-coded default in memory).
// Note that the OCI runtime is hard-set to `crun` if we're running on a system
// with cgroupsv2. Other OCI runtimes are not yet supporting cgroupsv2. This
// might change in the future.
func NewConfig(userConfigPath string) (*Config, error) {

	defaultConfig, err := DefaultConfig()
	if err != nil {
		return nil, errors.Wrapf(err, "error generating default config from memory")
	}

	// start from a default config from memory
	config := defaultConfig

	var configs []string
	// First, try to read the user-specified config
	if userConfigPath != "" {
		configs = []string{userConfigPath}
	} else {
		// Now, check if the user can access system configs and merge them if needed.
		configs, err = systemConfigs()
		if err != nil {
			return nil, errors.Wrapf(err, "error finding config on system")
		}

	}

	for _, path := range configs {
		systemConfig, err := ReadConfigFromFile(path)
		if err != nil && !os.IsNotExist(err) {
			return nil, errors.Wrapf(err, "error reading system config %q", path)
		}
		// Merge the it into the config. Any unset field in config will be
		// over-written by the systemConfig.
		if err := config.mergeConfig(systemConfig); err != nil {
			return nil, errors.Wrapf(err, "error merging system config")
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

func (t *tomlConfig) toConfig(c *Config) {
	c.ContainersConfig = t.Containers.ContainersConfig
	c.LibpodConfig = t.Libpod.LibpodConfig
	c.NetworkConfig = t.Network.NetworkConfig
}

func (t *tomlConfig) fromConfig(c *Config) {
	t.Containers.ContainersConfig = c.ContainersConfig
	t.Libpod.LibpodConfig = c.LibpodConfig
	t.Network.NetworkConfig = c.NetworkConfig
}

// ReadConfigFromFile reads the specified config file at `path` and attempts to
// unmarshal its content into a Config.
func ReadConfigFromFile(path string) (*Config, error) {
	var config Config
	t := new(tomlConfig)
	t.fromConfig(&config)

	configBytes, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	logrus.Debugf("Reading configuration file %q", path)
	_, err = toml.Decode(string(configBytes), t)
	if err != nil {
		return nil, fmt.Errorf("unable to decode configuration %v: %v", path, err)
	}
	t.toConfig(&config)

	// For the sake of backwards compat we need to check if the config fields
	// with *Set suffix are set in the config.  Note that the storage-related
	// fields are NOT set in the config here but in the storage.conf OR directly
	// by the user.
	if config.VolumePath != "" {
		config.VolumePathSet = true
	}
	if config.StaticDir != "" {
		config.StaticDirSet = true
	}
	if config.TmpDir != "" {
		config.TmpDirSet = true
	}

	return &config, err
}

func systemConfigs() ([]string, error) {
	configs := []string{}
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
	if !unshare.IsRootless() || c.CgroupManager != SystemdCgroupsManager {
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
		c.CgroupManager = CgroupfsCgroupsManager
	}
}

func (c *Config) addCAPPrefix() {
	toCAPPrefixed := func(cap string) string {
		if !strings.HasPrefix(strings.ToLower(cap), "cap_") {
			return "CAP_" + strings.ToUpper(cap)
		}
		return cap
	}
	for i, cap := range c.ContainersConfig.DefaultCapabilities {
		c.ContainersConfig.DefaultCapabilities[i] = toCAPPrefixed(cap)
	}
}

// Validate is the main entry point for library configuration validation.
// The parameter `onExecution` specifies if the validation should include
// execution checks. It returns an `error` on validation failure, otherwise
// `nil`.
func (c *Config) Validate(onExecution bool) error {

	if err := c.ContainersConfig.Validate(); err != nil {
		return errors.Wrapf(err, "containers config")
	}

	if !unshare.IsRootless() {
		if err := c.NetworkConfig.Validate(onExecution); err != nil {
			return errors.Wrapf(err, "network config")
		}
	}
	if !c.EnableLabeling {
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

	if !c.StorageConfigRunRootSet && dbConfig.StorageTmp != "" {
		if c.StorageConfig.RunRoot != dbConfig.StorageTmp &&
			c.StorageConfig.RunRoot != "" {
			logrus.Debugf("Overriding run root %q with %q from database",
				c.StorageConfig.RunRoot, dbConfig.StorageTmp)
		}
		c.StorageConfig.RunRoot = dbConfig.StorageTmp
	}

	if !c.StorageConfigGraphRootSet && dbConfig.StorageRoot != "" {
		if c.StorageConfig.GraphRoot != dbConfig.StorageRoot &&
			c.StorageConfig.GraphRoot != "" {
			logrus.Debugf("Overriding graph root %q with %q from database",
				c.StorageConfig.GraphRoot, dbConfig.StorageRoot)
		}
		c.StorageConfig.GraphRoot = dbConfig.StorageRoot
	}

	if !c.StorageConfigGraphDriverNameSet && dbConfig.GraphDriver != "" {
		if c.StorageConfig.GraphDriverName != dbConfig.GraphDriver &&
			c.StorageConfig.GraphDriverName != "" {
			logrus.Errorf("User-selected graph driver %q overwritten by graph driver %q from database - delete libpod local files to resolve",
				c.StorageConfig.GraphDriverName, dbConfig.GraphDriver)
		}
		c.StorageConfig.GraphDriverName = dbConfig.GraphDriver
	}

	if !c.StaticDirSet && dbConfig.LibpodRoot != "" {
		if c.StaticDir != dbConfig.LibpodRoot && c.StaticDir != "" {
			logrus.Debugf("Overriding static dir %q with %q from database", c.StaticDir, dbConfig.LibpodRoot)
		}
		c.StaticDir = dbConfig.LibpodRoot
	}

	if !c.TmpDirSet && dbConfig.LibpodTmp != "" {
		if c.TmpDir != dbConfig.LibpodTmp && c.TmpDir != "" {
			logrus.Debugf("Overriding tmp dir %q with %q from database", c.TmpDir, dbConfig.LibpodTmp)
		}
		c.TmpDir = dbConfig.LibpodTmp
		c.EventsLogFilePath = filepath.Join(dbConfig.LibpodTmp, "events", "events.log")
	}

	if !c.VolumePathSet && dbConfig.VolumePath != "" {
		if c.VolumePath != dbConfig.VolumePath && c.VolumePath != "" {
			logrus.Debugf("Overriding volume path %q with %q from database", c.VolumePath, dbConfig.VolumePath)
		}
		c.VolumePath = dbConfig.VolumePath
	}
	return nil
}

// FindConmon iterates over (*Config).ConmonPath and returns the path to first
// (version) matching conmon binary. If non is found, we try to do a path lookup
// of "conmon".
func (c *Config) FindConmon() (string, error) {
	foundOutdatedConmon := false
	for _, path := range c.ConmonPath {
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
		c.ConmonPath)
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
