package config

/* libpodConfig.go contains deprecated functionality and should not be used any longer */

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/containers/common/pkg/cgroupv2"
	"github.com/containers/storage/pkg/unshare"
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
	RuntimeSupportsNoCgroups []string `toml:"runtime_supports_nocgroupv2,omitempty"`

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
	// upgrade to Fedora 31 to change the default OCI runtime for cgroupv2v2.
	CgroupCheck bool `toml:"cgroup_check,omitempty"`
}

// newLibpodConfig creates a new ConfigFromLibpod and converts it to Config.
// Depending if we're running as root or rootless, we then merge the system configuration followed
// by merging the default config (hard-coded default in memory).
// Note that the OCI runtime is hard-set to `crun` if we're running on a system
// with cgroupv2v2. Other OCI runtimes are not yet supporting cgroupv2v2. This
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
	// TODO Once runc has support for cgroupv2, this function should be removed.
	if !config.CgroupCheck && unshare.IsRootless() {
		cgroup2, err := cgroupv2.Enabled()
		if err != nil {
			return err
		}
		if cgroup2 {
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

	// hard code EventsLogger to "file" to match older podman versions.
	if config.EventsLogger != "file" {
		logrus.Debugf("Ignoring libpod.conf EventsLogger setting %q. Use %q if you want to change this setting and remove libpod.conf files.", Path(), config.EventsLogger)
		config.EventsLogger = "file"
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
			// TODO: Raise to Warnf, when Podman is updated to
			// remove libpod.conf by default
			logrus.Debugf("Found deprecated file %s, please remove. Use %s to override defaults.\n", Path(), containersConfPath)
			return []string{path}, nil
		}
		return nil, err
	}

	configs := []string{}
	if _, err := os.Stat(_rootConfigPath); err == nil {
		// TODO: Raise to Warnf, when Podman is updated to
		// remove libpod.conf by default
		logrus.Debugf("Found deprecated file %s, please remove. Use %s to override defaults.\n", _rootConfigPath, OverrideContainersConfig)
		configs = append(configs, _rootConfigPath)
	}
	if _, err := os.Stat(_rootOverrideConfigPath); err == nil {
		// TODO: Raise to Warnf, when Podman is updated to
		// remove libpod.conf by default
		logrus.Debugf("Found deprecated file %s, please remove. Use %s to override defaults.\n", _rootOverrideConfigPath, OverrideContainersConfig)
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
		InitPath:       c.Containers.InitPath,
		MaxLogSize:     c.Containers.LogSizeMax,
		EnableLabeling: c.Containers.EnableLabeling,

		SetOptions:               c.Engine.SetOptions,
		VolumePath:               c.Engine.VolumePath,
		ImageDefaultTransport:    c.Engine.ImageDefaultTransport,
		OCIRuntime:               c.Engine.OCIRuntime,
		OCIRuntimes:              c.Engine.OCIRuntimes,
		RuntimeSupportsJSON:      c.Engine.RuntimeSupportsJSON,
		RuntimeSupportsNoCgroups: c.Engine.RuntimeSupportsNoCgroups,
		RuntimePath:              c.Engine.RuntimePath,
		ConmonPath:               c.Engine.ConmonPath,
		ConmonEnvVars:            c.Engine.ConmonEnvVars,
		CgroupManager:            c.Engine.CgroupManager,
		StaticDir:                c.Engine.StaticDir,
		TmpDir:                   c.Engine.TmpDir,
		NoPivotRoot:              c.Engine.NoPivotRoot,
		HooksDir:                 c.Engine.HooksDir,
		Namespace:                c.Engine.Namespace,
		InfraImage:               c.Engine.InfraImage,
		InfraCommand:             c.Engine.InfraCommand,
		EnablePortReservation:    c.Engine.EnablePortReservation,
		NetworkCmdPath:           c.Engine.NetworkCmdPath,
		NumLocks:                 c.Engine.NumLocks,
		LockType:                 c.Engine.LockType,
		EventsLogger:             c.Engine.EventsLogger,
		EventsLogFilePath:        c.Engine.EventsLogFilePath,
		DetachKeys:               c.Engine.DetachKeys,
		SDNotify:                 c.Engine.SDNotify,
		CgroupCheck:              c.Engine.CgroupCheck,
		SignaturePolicyPath:      c.Engine.SignaturePolicyPath,

		CNIConfigDir:      c.Network.NetworkConfigDir,
		CNIPluginDir:      c.Network.CNIPluginDirs,
		CNIDefaultNetwork: c.Network.DefaultNetwork,
	}
}

func (c *Config) libpodToContainersConfig(libpodConf *ConfigFromLibpod) {

	if libpodConf.InitPath != "" {
		c.Containers.InitPath = libpodConf.InitPath
	}
	c.Containers.LogSizeMax = libpodConf.MaxLogSize
	c.Containers.EnableLabeling = libpodConf.EnableLabeling

	if libpodConf.SignaturePolicyPath != "" {
		c.Engine.SignaturePolicyPath = libpodConf.SignaturePolicyPath
	}
	c.Engine.SetOptions = libpodConf.SetOptions
	if libpodConf.VolumePath != "" {
		c.Engine.VolumePath = libpodConf.VolumePath
	}
	if libpodConf.ImageDefaultTransport != "" {
		c.Engine.ImageDefaultTransport = libpodConf.ImageDefaultTransport
	}
	if libpodConf.OCIRuntime != "" {
		c.Engine.OCIRuntime = libpodConf.OCIRuntime
	}
	c.Engine.OCIRuntimes = libpodConf.OCIRuntimes
	c.Engine.RuntimeSupportsJSON = libpodConf.RuntimeSupportsJSON
	c.Engine.RuntimeSupportsNoCgroups = libpodConf.RuntimeSupportsNoCgroups
	c.Engine.RuntimePath = libpodConf.RuntimePath
	c.Engine.ConmonPath = libpodConf.ConmonPath
	c.Engine.ConmonEnvVars = libpodConf.ConmonEnvVars
	if libpodConf.CgroupManager != "" {
		c.Engine.CgroupManager = libpodConf.CgroupManager
	}
	if libpodConf.StaticDir != "" {
		c.Engine.StaticDir = libpodConf.StaticDir
	}
	if libpodConf.TmpDir != "" {
		c.Engine.TmpDir = libpodConf.TmpDir
	}
	c.Engine.NoPivotRoot = libpodConf.NoPivotRoot
	c.Engine.HooksDir = libpodConf.HooksDir
	if libpodConf.Namespace != "" {
		c.Engine.Namespace = libpodConf.Namespace
	}
	if libpodConf.InfraImage != "" {
		c.Engine.InfraImage = libpodConf.InfraImage
	}
	if libpodConf.InfraCommand != "" {
		c.Engine.InfraCommand = libpodConf.InfraCommand
	}

	c.Engine.EnablePortReservation = libpodConf.EnablePortReservation
	if libpodConf.NetworkCmdPath != "" {
		c.Engine.NetworkCmdPath = libpodConf.NetworkCmdPath
	}
	c.Engine.NumLocks = libpodConf.NumLocks
	c.Engine.LockType = libpodConf.LockType
	if libpodConf.EventsLogger != "" {
		c.Engine.EventsLogger = libpodConf.EventsLogger
	}
	if libpodConf.EventsLogFilePath != "" {
		c.Engine.EventsLogFilePath = libpodConf.EventsLogFilePath
	}
	if libpodConf.DetachKeys != "" {
		c.Engine.DetachKeys = libpodConf.DetachKeys
	}
	c.Engine.SDNotify = libpodConf.SDNotify
	c.Engine.CgroupCheck = libpodConf.CgroupCheck

	if libpodConf.CNIConfigDir != "" {
		c.Network.NetworkConfigDir = libpodConf.CNIConfigDir
	}
	c.Network.CNIPluginDirs = libpodConf.CNIPluginDir
	if libpodConf.CNIDefaultNetwork != "" {
		c.Network.DefaultNetwork = libpodConf.CNIDefaultNetwork
	}
}
