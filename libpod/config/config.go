package config

import (
	"bytes"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/containers/libpod/pkg/util"
	"github.com/containers/storage"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	// _defaultTransport is a prefix that we apply to an image name to check
	// docker hub first for the image.
	_defaultTransport = "docker://"

	// _rootlessConfigPath is the path to the rootless libpod.conf in $HOME.
	_rootlessConfigPath = ".config/containers/libpod.conf"

	// _conmonMinMajorVersion is the major version required for conmon.
	_conmonMinMajorVersion = 2

	// _conmonMinMinorVersion is the minor version required for conmon.
	_conmonMinMinorVersion = 0

	// _conmonMinPatchVersion is the sub-minor version required for conmon.
	_conmonMinPatchVersion = 1

	// _conmonVersionFormatErr is used when the expected versio-format of conmon
	// has changed.
	_conmonVersionFormatErr = "conmon version changed format"

	// InstallPrefix is the prefix where podman will be installed.
	// It can be overridden at build time.
	_installPrefix = "/usr"

	// EtcDir is the sysconfdir where podman should look for system config files.
	// It can be overridden at build time.
	_etcDir = "/etc"

	// SeccompDefaultPath defines the default seccomp path.
	SeccompDefaultPath = _installPrefix + "/share/containers/seccomp.json"

	// SeccompOverridePath if this exists it overrides the default seccomp path.
	SeccompOverridePath = _etcDir + "/crio/seccomp.json"

	// _rootConfigPath is the path to the libpod configuration file
	// This file is loaded to replace the builtin default config before
	// runtime options (e.g. WithStorageConfig) are applied.
	// If it is not present, the builtin default config is used instead
	// This path can be overridden when the runtime is created by using
	// NewRuntimeFromConfig() instead of NewRuntime().
	_rootConfigPath = _installPrefix + "/share/containers/libpod.conf"

	// _rootOverrideConfigPath is the path to an override for the default libpod
	// configuration file. If OverrideConfigPath exists, it will be used in
	// place of the configuration file pointed to by ConfigPath.
	_rootOverrideConfigPath = _etcDir + "/containers/libpod.conf"
)

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

	// VolumePathSet indicates if the VolumePath has been explicitly set by the
	// config or by the user. It's required to guarantee backwards compatibility
	// with older versions of libpod for which we must query the database
	// configuration. Not included in the on-disk config.
	VolumePathSet bool `toml:"-"`

	// StaticDirSet indicates if the StaticDir has been explicitly set by the
	// config or by the user. It's required to guarantee backwards compatibility
	// with older versions of libpod for which we must query the database
	// configuration. Not included in the on-disk config.
	StaticDirSet bool `toml:"-"`

	// TmpDirSet indicates if the TmpDir has been explicitly set by the config
	// or by the user. It's required to guarantee backwards compatibility with
	// older versions of libpod for which we must query the database
	// configuration. Not included in the on-disk config.
	TmpDirSet bool `toml:"-"`
}

// Config contains configuration options used to set up a libpod runtime
type Config struct {
	// NOTE: when changing this struct, make sure to update (*Config).Merge().

	// SetOptions contains a subset of config options. It's used to indicate if
	// a given option has either been set by the user or by a parsed libpod
	// configuration file.  If not, the corresponding option might be
	// overwritten by values from the database.  This behavior guarantess
	// backwards compat with older version of libpod and Podman.
	SetOptions

	// StateType is the type of the backing state store. Avoid using multiple
	// values for this with the same containers/storage configuration on the
	// same system. Different state types do not interact, and each will see a
	// separate set of containers, which may cause conflicts in
	// containers/storage. As such this is not exposed via the config file.
	StateType define.RuntimeStateStore `toml:"-"`

	// StorageConfig is the configuration used by containers/storage Not
	// included in the on-disk config, use the dedicated containers/storage
	// configuration file instead.
	StorageConfig storage.StoreOptions `toml:"-"`

	// VolumePath is the default location that named volumes will be created
	// under. This convention is followed by the default volume driver, but
	// may not be by other drivers.
	VolumePath string `toml:"volume_path"`

	// ImageDefaultTransport is the default transport method used to fetch
	// images.
	ImageDefaultTransport string `toml:"image_default_transport"`

	// SignaturePolicyPath is the path to a signature policy to use for
	// validating images. If left empty, the containers/image default signature
	// policy will be used.
	SignaturePolicyPath string `toml:"signature_policy_path,omitempty"`

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

	// RuntimePath is the path to OCI runtime binary for launching containers.
	// The first path pointing to a valid file will be used This is used only
	// when there are no OCIRuntime/OCIRuntimes defined.  It is used only to be
	// backward compatible with older versions of Podman.
	RuntimePath []string `toml:"runtime_path"`

	// ConmonPath is the path to the Conmon binary used for managing containers.
	// The first path pointing to a valid file will be used.
	ConmonPath []string `toml:"conmon_path"`

	// ConmonEnvVars are environment variables to pass to the Conmon binary
	// when it is launched.
	ConmonEnvVars []string `toml:"conmon_env_vars"`

	// CGroupManager is the CGroup Manager to use Valid values are "cgroupfs"
	// and "systemd".
	CgroupManager string `toml:"cgroup_manager"`

	// InitPath is the path to the container-init binary.
	InitPath string `toml:"init_path"`

	// StaticDir is the path to a persistent directory to store container
	// files.
	StaticDir string `toml:"static_dir"`

	// TmpDir is the path to a temporary directory to store per-boot container
	// files. Must be stored in a tmpfs.
	TmpDir string `toml:"tmp_dir"`

	// MaxLogSize is the maximum size of container logfiles.
	MaxLogSize int64 `toml:"max_log_size,omitempty"`

	// NoPivotRoot sets whether to set no-pivot-root in the OCI runtime.
	NoPivotRoot bool `toml:"no_pivot_root"`

	// CNIConfigDir sets the directory where CNI configuration files are
	// stored.
	CNIConfigDir string `toml:"cni_config_dir"`

	// CNIPluginDir sets a number of directories where the CNI network
	// plugins can be located.
	CNIPluginDir []string `toml:"cni_plugin_dir"`

	// CNIDefaultNetwork is the network name of the default CNI network
	// to attach pods to.
	CNIDefaultNetwork string `toml:"cni_default_network,omitempty"`

	// HooksDir holds paths to the directories containing hooks
	// configuration files. When the same filename is present in in
	// multiple directories, the file in the directory listed last in
	// this slice takes precedence.
	HooksDir []string `toml:"hooks_dir"`

	// DefaultMountsFile is the path to the default mounts file for testing
	// purposes only.
	DefaultMountsFile string `toml:"-"`

	// Namespace is the libpod namespace to use. Namespaces are used to create
	// scopes to separate containers and pods in the state. When namespace is
	// set, libpod will only view containers and pods in the same namespace. All
	// containers and pods created will default to the namespace set here. A
	// namespace of "", the empty string, is equivalent to no namespace, and all
	// containers and pods will be visible. The default namespace is "".
	Namespace string `toml:"namespace,omitempty"`

	// InfraImage is the image a pod infra container will use to manage
	// namespaces.
	InfraImage string `toml:"infra_image"`

	// InfraCommand is the command run to start up a pod infra container.
	InfraCommand string `toml:"infra_command"`

	// EnablePortReservation determines whether libpod will reserve ports on the
	// host when they are forwarded to containers. When enabled, when ports are
	// forwarded to containers, they are held open by conmon as long as the
	// container is running, ensuring that they cannot be reused by other
	// programs on the host. However, this can cause significant memory usage if
	// a container has many ports forwarded to it. Disabling this can save
	// memory.
	EnablePortReservation bool `toml:"enable_port_reservation"`

	// EnableLabeling indicates whether libpod will support container labeling.
	EnableLabeling bool `toml:"label"`

	// NetworkCmdPath is the path to the slirp4netns binary.
	NetworkCmdPath string `toml:"network_cmd_path"`

	// NumLocks is the number of locks to make available for containers and
	// pods.
	NumLocks uint32 `toml:"num_locks,omitempty"`

	// LockType is the type of locking to use.
	LockType string `toml:"lock_type,omitempty"`

	// EventsLogger determines where events should be logged.
	EventsLogger string `toml:"events_logger"`

	// EventsLogFilePath is where the events log is stored.
	EventsLogFilePath string `toml:"events_logfile_path"`

	//DetachKeys is the sequence of keys used to detach a container.
	DetachKeys string `toml:"detach_keys"`

	// SDNotify tells Libpod to allow containers to notify the host systemd of
	// readiness using the SD_NOTIFY mechanism.
	SDNotify bool
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

// readConfigFromFile reads the specified config file at `path` and attempts to
// unmarshal its content into a Config.
func readConfigFromFile(path string) (*Config, error) {
	var config Config

	configBytes, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	logrus.Debugf("Reading configuration file %q", path)
	err = toml.Unmarshal(configBytes, &config)

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

// Write decodes the config as TOML and writes it to the specified path.
func (c *Config) Write(path string) error {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return errors.Wrapf(err, "error opening config file %q", path)
	}

	buffer := new(bytes.Buffer)
	if err := toml.NewEncoder(buffer).Encode(c); err != nil {
		return errors.Wrapf(err, "error encoding config")
	}

	if _, err := f.WriteString(buffer.String()); err != nil {
		return errors.Wrapf(err, "error writing config %q", path)
	}
	return err
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
		return "", errors.Wrapf(define.ErrConmonOutdated,
			"please update to v%d.%d.%d or later",
			_conmonMinMajorVersion, _conmonMinMinorVersion, _conmonMinPatchVersion)
	}

	return "", errors.Wrapf(define.ErrInvalidArg,
		"could not find a working conmon binary (configured options: %v)",
		c.ConmonPath)
}

// probeConmon calls conmon --version and verifies it is a new enough version for
// the runtime expectations podman currently has.
func probeConmon(conmonBinary string) error {
	cmd := exec.Command(conmonBinary, "--version")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return err
	}
	r := regexp.MustCompile(`^conmon version (?P<Major>\d+).(?P<Minor>\d+).(?P<Patch>\d+)`)

	matches := r.FindStringSubmatch(out.String())
	if len(matches) != 4 {
		return errors.Wrap(err, _conmonVersionFormatErr)
	}
	major, err := strconv.Atoi(matches[1])
	if err != nil {
		return errors.Wrap(err, _conmonVersionFormatErr)
	}
	if major < _conmonMinMajorVersion {
		return define.ErrConmonOutdated
	}
	if major > _conmonMinMajorVersion {
		return nil
	}

	minor, err := strconv.Atoi(matches[2])
	if err != nil {
		return errors.Wrap(err, _conmonVersionFormatErr)
	}
	if minor < _conmonMinMinorVersion {
		return define.ErrConmonOutdated
	}
	if minor > _conmonMinMinorVersion {
		return nil
	}

	patch, err := strconv.Atoi(matches[3])
	if err != nil {
		return errors.Wrap(err, _conmonVersionFormatErr)
	}
	if patch < _conmonMinPatchVersion {
		return define.ErrConmonOutdated
	}
	if patch > _conmonMinPatchVersion {
		return nil
	}

	return nil
}

// NewConfig creates a new Config. It starts with an empty config and, if
// specified, merges the config at `userConfigPath` path.  Depending if we're
// running as root or rootless, we then merge the system configuration followed
// by merging the default config (hard-coded default in memory).
//
// Note that the OCI runtime is hard-set to `crun` if we're running on a system
// with cgroupsv2. Other OCI runtimes are not yet supporting cgroupsv2. This
// might change in the future.
func NewConfig(userConfigPath string) (*Config, error) {
	config := &Config{} // start with an empty config

	// First, try to read the user-specified config
	if userConfigPath != "" {
		var err error
		config, err = readConfigFromFile(userConfigPath)
		if err != nil {
			return nil, errors.Wrapf(err, "error reading user config %q", userConfigPath)
		}
	}

	// Now, check if the user can access system configs and merge them if needed.
	if configs, err := systemConfigs(); err != nil {
		return nil, errors.Wrapf(err, "error finding config on system")
	} else {
		for _, path := range configs {
			systemConfig, err := readConfigFromFile(path)
			if err != nil {
				return nil, errors.Wrapf(err, "error reading system config %q", path)
			}
			// Merge the it into the config. Any unset field in config will be
			// over-written by the systemConfig.
			if err := config.mergeConfig(systemConfig); err != nil {
				return nil, errors.Wrapf(err, "error merging system config")
			}
			logrus.Debugf("Merged system config %q: %v", path, config)
		}
	}

	// Finally, create a default config from memory and forcefully merge it into
	// the config. This way we try to make sure that all fields are properly set
	// and that user AND system config can partially set.
	if defaultConfig, err := defaultConfigFromMemory(); err != nil {
		return nil, errors.Wrapf(err, "error generating default config from memory")
	} else {
		if err := config.mergeConfig(defaultConfig); err != nil {
			return nil, errors.Wrapf(err, "error merging default config from memory")
		}
	}

	// Relative paths can cause nasty bugs, because core paths we use could
	// shift between runs (or even parts of the program - the OCI runtime
	// uses a different working directory than we do, for example.
	if !filepath.IsAbs(config.StaticDir) {
		return nil, errors.Wrapf(define.ErrInvalidArg, "static directory must be an absolute path - instead got %q", config.StaticDir)
	}
	if !filepath.IsAbs(config.TmpDir) {
		return nil, errors.Wrapf(define.ErrInvalidArg, "temporary directory must be an absolute path - instead got %q", config.TmpDir)
	}
	if !filepath.IsAbs(config.VolumePath) {
		return nil, errors.Wrapf(define.ErrInvalidArg, "volume path must be an absolute path - instead got %q", config.VolumePath)
	}

	// Check if we need to switch to cgroupfs on rootless.
	config.checkCgroupsAndAdjustConfig()

	return config, nil
}

func rootlessConfigPath() (string, error) {
	home, err := util.HomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, _rootlessConfigPath), nil
}

func systemConfigs() ([]string, error) {
	if rootless.IsRootless() {
		path, err := rootlessConfigPath()
		if err != nil {
			return nil, err
		}
		if _, err := os.Stat(path); err == nil {
			return []string{path}, nil
		}
		return nil, err
	}

	configs := []string{}
	if _, err := os.Stat(_rootOverrideConfigPath); err == nil {
		configs = append(configs, _rootOverrideConfigPath)
	}
	if _, err := os.Stat(_rootConfigPath); err == nil {
		configs = append(configs, _rootConfigPath)
	}
	return configs, nil
}

// checkCgroupsAndAdjustConfig checks if we're running rootless with the systemd
// cgroup manager. In case the user session isn't available, we're switching the
// cgroup manager to cgroupfs.  Note, this only applies to rootless.
func (c *Config) checkCgroupsAndAdjustConfig() {
	if !rootless.IsRootless() || c.CgroupManager != define.SystemdCgroupsManager {
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
		logrus.Warningf("Alternatively, you can enable lingering with: `loginctl enable-linger %d` (possibly as root)", rootless.GetRootlessUID())
		logrus.Warningf("Falling back to --cgroup-manager=cgroupfs")
		c.CgroupManager = define.CgroupfsCgroupsManager
	}
}
