package config

/* libpodConfig.go contains deprecated functionality and should not be used any longer */

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/containers/common/pkg/unshare"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	// _rootlessConfigPath is the path to the rootless libpod.conf in $HOME.
	_rootlessConfigPath = ".config/containers/libpod.conf"

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

// ConfigFromLibpod contains configuration options used to set up a libpod runtime
type ConfigFromLibpod struct {
	// NOTE: when changing this struct, make sure to update (*Config).Merge().

	// SetOptions contains a subset of config options. It's used to indicate if
	// a given option has either been set by the user or by a parsed libpod
	// configuration file.  If not, the corresponding option might be
	// overwritten by values from the database.  This behavior guarantees
	// backwards compat with older version of libpod and Podman.
	SetOptions

	// VolumePath is the default location that named volumes will be created
	// under. This convention is followed by the default volume driver, but
	// may not be by other drivers.
	VolumePath string `toml:"volume_path,omitempty"`

	// ImageDefaultTransport is the default transport method used to fetch
	// images.
	ImageDefaultTransport string `toml:"image_default_transport,omitempty"`

	// SignaturePolicyPath is the path to a signature policy to use for
	// validating images. If left empty, the containers/image default signature
	// policy will be used.
	SignaturePolicyPath string `toml:"signature_policy_path,omitempty"`

	// OCIRuntime is the OCI runtime to use.
	OCIRuntime string `toml:"runtime,omitempty"`

	// OCIRuntimes are the set of configured OCI runtimes (default is runc).
	OCIRuntimes map[string][]string `toml:"runtimes,omitempty"`

	// RuntimeSupportsJSON is the list of the OCI runtimes that support
	// --format=json.
	RuntimeSupportsJSON []string `toml:"runtime_supports_json,omitempty"`

	// RuntimeSupportsNoCgroups is a list of OCI runtimes that support
	// running containers without CGroups.
	RuntimeSupportsNoCgroups []string `toml:"runtime_supports_nocgroups,omitempty"`

	// RuntimePath is the path to OCI runtime binary for launching containers.
	// The first path pointing to a valid file will be used This is used only
	// when there are no OCIRuntime/OCIRuntimes defined.  It is used only to be
	// backward compatible with older versions of Podman.
	RuntimePath []string `toml:"runtime_path,omitempty"`

	// ConmonPath is the path to the Conmon binary used for managing containers.
	// The first path pointing to a valid file will be used.
	ConmonPath []string `toml:"conmon_path,omitempty"`

	// ConmonEnvVars are environment variables to pass to the Conmon binary
	// when it is launched.
	ConmonEnvVars []string `toml:"conmon_env_vars,omitempty"`

	// CGroupManager is the CGroup Manager to use Valid values are "cgroupfs"
	// and "systemd".
	CgroupManager string `toml:"cgroup_manager,omitempty"`

	// InitPath is the path to the container-init binary.
	InitPath string `toml:"init_path,omitempty"`

	// StaticDir is the path to a persistent directory to store container
	// files.
	StaticDir string `toml:"static_dir,omitempty"`

	// TmpDir is the path to a temporary directory to store per-boot container
	// files. Must be stored in a tmpfs.
	TmpDir string `toml:"tmp_dir,omitempty"`

	// MaxLogSize is the maximum size of container logfiles.
	MaxLogSize int64 `toml:"max_log_size,omitempty"`

	// NoPivotRoot sets whether to set no-pivot-root in the OCI runtime.
	NoPivotRoot bool `toml:"no_pivot_root,omitempty"`

	// CNIConfigDir sets the directory where CNI configuration files are
	// stored.
	CNIConfigDir string `toml:"cni_config_dir,omitempty"`

	// CNIPluginDir sets a number of directories where the CNI network
	// plugins can be located.
	CNIPluginDir []string `toml:"cni_plugin_dir,omitempty"`

	// CNIDefaultNetwork is the network name of the default CNI network
	// to attach pods to.
	CNIDefaultNetwork string `toml:"cni_default_network,omitempty"`

	// HooksDir holds paths to the directories containing hooks
	// configuration files. When the same filename is present in in
	// multiple directories, the file in the directory listed last in
	// this slice takes precedence.
	HooksDir []string `toml:"hooks_dir,omitempty"`

	// Namespace is the libpod namespace to use. Namespaces are used to create
	// scopes to separate containers and pods in the state. When namespace is
	// set, libpod will only view containers and pods in the same namespace. All
	// containers and pods created will default to the namespace set here. A
	// namespace of "", the empty string, is equivalent to no namespace, and all
	// containers and pods will be visible. The default namespace is "".
	Namespace string `toml:"namespace,omitempty"`

	// InfraImage is the image a pod infra container will use to manage
	// namespaces.
	InfraImage string `toml:"infra_image,omitempty"`

	// InfraCommand is the command run to start up a pod infra container.
	InfraCommand string `toml:"infra_command,omitempty"`

	// EnablePortReservation determines whether libpod will reserve ports on the
	// host when they are forwarded to containers. When enabled, when ports are
	// forwarded to containers, they are held open by conmon as long as the
	// container is running, ensuring that they cannot be reused by other
	// programs on the host. However, this can cause significant memory usage if
	// a container has many ports forwarded to it. Disabling this can save
	// memory.
	EnablePortReservation bool `toml:"enable_port_reservation,omitempty"`

	// EnableLabeling indicates whether libpod will support container labeling.
	EnableLabeling bool `toml:"label,omitempty"`

	// NetworkCmdPath is the path to the slirp4netns binary.
	NetworkCmdPath string `toml:"network_cmd_path,omitempty"`

	// NumLocks is the number of locks to make available for containers and
	// pods.
	NumLocks uint32 `toml:"num_locks,omitempty"`

	// LockType is the type of locking to use.
	LockType string `toml:"lock_type,omitempty"`

	// EventsLogger determines where events should be logged.
	EventsLogger string `toml:"events_logger,omitempty"`

	// EventsLogFilePath is where the events log is stored.
	EventsLogFilePath string `toml:"events_logfile_path,omitempty"`

	//DetachKeys is the sequence of keys used to detach a container.
	DetachKeys string `toml:"detach_keys,omitempty"`

	// SDNotify tells Libpod to allow containers to notify the host systemd of
	// readiness using the SD_NOTIFY mechanism.
	SDNotify bool `toml:",omitempty"`

	// CgroupCheck indicates the configuration has been rewritten after an
	// upgrade to Fedora 31 to change the default OCI runtime for cgroupsv2.
	CgroupCheck bool `toml:"cgroup_check,omitempty"`
}

// newLibpodConfig creates a new ConfigFromLibpod and converts it to Config.
// Depending if we're running as root or rootless, we then merge the system configuration followed
// by merging the default config (hard-coded default in memory).
// Note that the OCI runtime is hard-set to `crun` if we're running on a system
// with cgroupsv2. Other OCI runtimes are not yet supporting cgroupsv2. This
// might change in the future.
func newLibpodConfig(c *Config) error {
	// Start with the default config and interatively merge
	// fields in the system configs.
	config := c.libpodConfig()

	// Now, check if the user can access system configs and merge them if needed.
	configs, err := systemLibpodConfigs()
	if err != nil {
		return errors.Wrapf(err, "error finding config on system")
	}

	for _, path := range configs {
		config, err = readLibpodConfigFromFile(path, config)
		if err != nil {
			return errors.Wrapf(err, "error reading system config %q", path)
		}
	}

	// Since runc does not currently support cgroupV2
	// Change to default crun on first running of libpod.conf
	// TODO Once runc has support for cgroups, this function should be removed.
	if !config.CgroupCheck && unshare.IsRootless() {
		cgroupsV2, err := isCgroup2UnifiedMode()
		if err != nil {
			return err
		}
		if cgroupsV2 {
			path, err := exec.LookPath("crun")
			if err != nil {
				// Can't find crun path so do nothing
				logrus.Warnf("Can not find crun package on the host, containers might fail to run on cgroup V2 systems without crun: %q", err)
			} else {
				config.CgroupCheck = true
				config.OCIRuntime = path
			}
		}
	}

	c.libpodToContainersConfig(config)

	return nil
}

// readConfigFromFile reads the specified config file at `path` and attempts to
// unmarshal its content into a Config. The config param specifies the previous
// default config. If the path, only specifies a few fields in the Toml file
// the defaults from the config parameter will be used for all other fields.
func readLibpodConfigFromFile(path string, config *ConfigFromLibpod) (*ConfigFromLibpod, error) {
	logrus.Debugf("Reading configuration file %q", path)
	_, err := toml.DecodeFile(path, config)
	if err != nil {
		return nil, fmt.Errorf("unable to decode configuration %v: %v", path, err)
	}

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

	return config, err
}

func systemLibpodConfigs() ([]string, error) {
	if unshare.IsRootless() {
		path, err := rootlessLibpodConfigPath()
		if err != nil {
			return nil, err
		}
		if _, err := os.Stat(path); err == nil {
			containersConfPath, err := rootlessConfigPath()
			if err != nil {
				containersConfPath = filepath.Join("$HOME", UserOverrideContainersConfig)
			}
			logrus.Warnf("Found deprecated file %s, please remove. Use %s to override defaults.\n", path, containersConfPath)
			return []string{path}, nil
		}
		return nil, err
	}

	configs := []string{}
	if _, err := os.Stat(_rootConfigPath); err == nil {
		logrus.Warnf("Found deprecated file %s, please remove. Use %s to override defaults.\n", _rootConfigPath, OverrideContainersConfig)
		configs = append(configs, _rootConfigPath)
	}
	if _, err := os.Stat(_rootOverrideConfigPath); err == nil {
		logrus.Warnf("Found deprecated file %s, please remove. Use %s to override defaults.\n", _rootOverrideConfigPath, OverrideContainersConfig)
		configs = append(configs, _rootOverrideConfigPath)
	}
	return configs, nil
}

func rootlessLibpodConfigPath() (string, error) {
	home, err := unshare.HomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, _rootlessConfigPath), nil
}

func (c *Config) libpodConfig() *ConfigFromLibpod {
	return &ConfigFromLibpod{
		SignaturePolicyPath: c.Containers.SignaturePolicyPath,
		InitPath:            c.Containers.InitPath,
		MaxLogSize:          c.Containers.LogSizeMax,
		EnableLabeling:      c.Containers.EnableLabeling,

		SetOptions:               c.Libpod.SetOptions,
		VolumePath:               c.Libpod.VolumePath,
		ImageDefaultTransport:    c.Libpod.ImageDefaultTransport,
		OCIRuntime:               c.Libpod.OCIRuntime,
		OCIRuntimes:              c.Libpod.OCIRuntimes,
		RuntimeSupportsJSON:      c.Libpod.RuntimeSupportsJSON,
		RuntimeSupportsNoCgroups: c.Libpod.RuntimeSupportsNoCgroups,
		RuntimePath:              c.Libpod.RuntimePath,
		ConmonPath:               c.Libpod.ConmonPath,
		ConmonEnvVars:            c.Libpod.ConmonEnvVars,
		CgroupManager:            c.Libpod.CgroupManager,
		StaticDir:                c.Libpod.StaticDir,
		TmpDir:                   c.Libpod.TmpDir,
		NoPivotRoot:              c.Libpod.NoPivotRoot,
		HooksDir:                 c.Libpod.HooksDir,
		Namespace:                c.Libpod.Namespace,
		InfraImage:               c.Libpod.InfraImage,
		InfraCommand:             c.Libpod.InfraCommand,
		EnablePortReservation:    c.Libpod.EnablePortReservation,
		NetworkCmdPath:           c.Libpod.NetworkCmdPath,
		NumLocks:                 c.Libpod.NumLocks,
		LockType:                 c.Libpod.LockType,
		EventsLogger:             c.Libpod.EventsLogger,
		EventsLogFilePath:        c.Libpod.EventsLogFilePath,
		DetachKeys:               c.Libpod.DetachKeys,
		SDNotify:                 c.Libpod.SDNotify,
		CgroupCheck:              c.Libpod.CgroupCheck,

		CNIConfigDir:      c.Network.NetworkConfigDir,
		CNIPluginDir:      c.Network.CNIPluginDirs,
		CNIDefaultNetwork: c.Network.DefaultNetwork,
	}
}

func (c *Config) libpodToContainersConfig(libpodConf *ConfigFromLibpod) {

	c.Containers.SignaturePolicyPath = libpodConf.SignaturePolicyPath
	c.Containers.InitPath = libpodConf.InitPath
	c.Containers.LogSizeMax = libpodConf.MaxLogSize
	c.Containers.EnableLabeling = libpodConf.EnableLabeling

	c.Libpod.SetOptions = libpodConf.SetOptions
	c.Libpod.VolumePath = libpodConf.VolumePath
	c.Libpod.ImageDefaultTransport = libpodConf.ImageDefaultTransport
	c.Libpod.OCIRuntime = libpodConf.OCIRuntime
	c.Libpod.OCIRuntimes = libpodConf.OCIRuntimes
	c.Libpod.RuntimeSupportsJSON = libpodConf.RuntimeSupportsJSON
	c.Libpod.RuntimeSupportsNoCgroups = libpodConf.RuntimeSupportsNoCgroups
	c.Libpod.RuntimePath = libpodConf.RuntimePath
	c.Libpod.ConmonPath = libpodConf.ConmonPath
	c.Libpod.ConmonEnvVars = libpodConf.ConmonEnvVars
	c.Libpod.CgroupManager = libpodConf.CgroupManager
	c.Libpod.StaticDir = libpodConf.StaticDir
	c.Libpod.TmpDir = libpodConf.TmpDir
	c.Libpod.NoPivotRoot = libpodConf.NoPivotRoot
	c.Libpod.HooksDir = libpodConf.HooksDir
	c.Libpod.Namespace = libpodConf.Namespace
	c.Libpod.InfraImage = libpodConf.InfraImage
	c.Libpod.InfraCommand = libpodConf.InfraCommand
	c.Libpod.EnablePortReservation = libpodConf.EnablePortReservation
	c.Libpod.NetworkCmdPath = libpodConf.NetworkCmdPath
	c.Libpod.NumLocks = libpodConf.NumLocks
	c.Libpod.LockType = libpodConf.LockType
	c.Libpod.EventsLogger = libpodConf.EventsLogger
	c.Libpod.EventsLogFilePath = libpodConf.EventsLogFilePath
	c.Libpod.DetachKeys = libpodConf.DetachKeys
	c.Libpod.SDNotify = libpodConf.SDNotify
	c.Libpod.CgroupCheck = libpodConf.CgroupCheck

	c.Network.NetworkConfigDir = libpodConf.CNIConfigDir
	c.Network.CNIPluginDirs = libpodConf.CNIPluginDir
	c.Network.DefaultNetwork = libpodConf.CNIDefaultNetwork
}
