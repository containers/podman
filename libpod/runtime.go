package libpod

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/BurntSushi/toml"
	is "github.com/containers/image/v4/storage"
	"github.com/containers/image/v4/types"
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/libpod/events"
	"github.com/containers/libpod/libpod/image"
	"github.com/containers/libpod/libpod/lock"
	"github.com/containers/libpod/pkg/cgroups"
	sysreg "github.com/containers/libpod/pkg/registries"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/containers/libpod/pkg/util"
	"github.com/containers/storage"
	"github.com/cri-o/ocicni/pkg/ocicni"
	"github.com/docker/docker/pkg/namesgenerator"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
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

var (
	// InstallPrefix is the prefix where podman will be installed.
	// It can be overridden at build time.
	installPrefix = "/usr"
	// EtcDir is the sysconfdir where podman should look for system config files.
	// It can be overridden at build time.
	etcDir = "/etc"

	// SeccompDefaultPath defines the default seccomp path
	SeccompDefaultPath = installPrefix + "/share/containers/seccomp.json"
	// SeccompOverridePath if this exists it overrides the default seccomp path
	SeccompOverridePath = etcDir + "/crio/seccomp.json"

	// ConfigPath is the path to the libpod configuration file
	// This file is loaded to replace the builtin default config before
	// runtime options (e.g. WithStorageConfig) are applied.
	// If it is not present, the builtin default config is used instead
	// This path can be overridden when the runtime is created by using
	// NewRuntimeFromConfig() instead of NewRuntime()
	ConfigPath = installPrefix + "/share/containers/libpod.conf"
	// OverrideConfigPath is the path to an override for the default libpod
	// configuration file. If OverrideConfigPath exists, it will be used in
	// place of the configuration file pointed to by ConfigPath.
	OverrideConfigPath = etcDir + "/containers/libpod.conf"

	// DefaultSHMLockPath is the default path for SHM locks
	DefaultSHMLockPath = "/libpod_lock"
	// DefaultRootlessSHMLockPath is the default path for rootless SHM locks
	DefaultRootlessSHMLockPath = "/libpod_rootless_lock"

	// DefaultDetachKeys is the default keys sequence for detaching a
	// container
	DefaultDetachKeys = "ctrl-p,ctrl-q"
)

// A RuntimeOption is a functional option which alters the Runtime created by
// NewRuntime
type RuntimeOption func(*Runtime) error

// Runtime is the core libpod runtime
type Runtime struct {
	config *RuntimeConfig

	state             State
	store             storage.Store
	storageService    *storageService
	imageContext      *types.SystemContext
	defaultOCIRuntime OCIRuntime
	ociRuntimes       map[string]OCIRuntime
	netPlugin         ocicni.CNIPlugin
	conmonPath        string
	imageRuntime      *image.Runtime
	lockManager       lock.Manager
	configuredFrom    *runtimeConfiguredFrom

	// doRenumber indicates that the runtime should perform a lock renumber
	// during initialization.
	// Once the runtime has been initialized and returned, this variable is
	// unused.
	doRenumber bool

	doMigrate bool
	// System migrate can move containers to a new runtime.
	// We make no promises that these migrated containers work on the new
	// runtime, though.
	migrateRuntime string

	// valid indicates whether the runtime is ready to use.
	// valid is set to true when a runtime is returned from GetRuntime(),
	// and remains true until the runtime is shut down (rendering its
	// storage unusable). When valid is false, the runtime cannot be used.
	valid bool
	lock  sync.RWMutex

	// mechanism to read and write even logs
	eventer events.Eventer

	// noStore indicates whether we need to interact with a store or not
	noStore bool
}

// RuntimeConfig contains configuration options used to set up the runtime
type RuntimeConfig struct {
	// StorageConfig is the configuration used by containers/storage
	// Not included in on-disk config, use the dedicated containers/storage
	// configuration file instead
	StorageConfig storage.StoreOptions `toml:"-"`
	// VolumePath is the default location that named volumes will be created
	// under. This convention is followed by the default volume driver, but
	// may not be by other drivers.
	VolumePath string `toml:"volume_path"`
	// ImageDefaultTransport is the default transport method used to fetch
	// images
	ImageDefaultTransport string `toml:"image_default_transport"`
	// SignaturePolicyPath is the path to a signature policy to use for
	// validating images
	// If left empty, the containers/image default signature policy will
	// be used
	SignaturePolicyPath string `toml:"signature_policy_path,omitempty"`
	// StateType is the type of the backing state store.
	// Avoid using multiple values for this with the same containers/storage
	// configuration on the same system. Different state types do not
	// interact, and each will see a separate set of containers, which may
	// cause conflicts in containers/storage
	// As such this is not exposed via the config file
	StateType RuntimeStateStore `toml:"-"`
	// OCIRuntime is the OCI runtime to use.
	OCIRuntime string `toml:"runtime"`
	// OCIRuntimes are the set of configured OCI runtimes (default is runc)
	OCIRuntimes map[string][]string `toml:"runtimes"`
	// RuntimeSupportsJSON is the list of the OCI runtimes that support
	// --format=json.
	RuntimeSupportsJSON []string `toml:"runtime_supports_json"`
	// RuntimeSupportsNoCgroups is a list of OCI runtimes that support
	// running containers without CGroups.
	RuntimeSupportsNoCgroups []string `toml:"runtime_supports_nocgroups"`
	// RuntimePath is the path to OCI runtime binary for launching
	// containers.
	// The first path pointing to a valid file will be used
	// This is used only when there are no OCIRuntime/OCIRuntimes defined.  It
	// is used only to be backward compatible with older versions of Podman.
	RuntimePath []string `toml:"runtime_path"`
	// ConmonPath is the path to the Conmon binary used for managing
	// containers
	// The first path pointing to a valid file will be used
	ConmonPath []string `toml:"conmon_path"`
	// ConmonEnvVars are environment variables to pass to the Conmon binary
	// when it is launched
	ConmonEnvVars []string `toml:"conmon_env_vars"`
	// CGroupManager is the CGroup Manager to use
	// Valid values are "cgroupfs" and "systemd"
	CgroupManager string `toml:"cgroup_manager"`
	// InitPath is the path to the container-init binary.
	InitPath string `toml:"init_path"`
	// StaticDir is the path to a persistent directory to store container
	// files
	StaticDir string `toml:"static_dir"`
	// TmpDir is the path to a temporary directory to store per-boot
	// container files
	// Must be stored in a tmpfs
	TmpDir string `toml:"tmp_dir"`
	// MaxLogSize is the maximum size of container logfiles
	MaxLogSize int64 `toml:"max_log_size,omitempty"`
	// NoPivotRoot sets whether to set no-pivot-root in the OCI runtime
	NoPivotRoot bool `toml:"no_pivot_root"`
	// CNIConfigDir sets the directory where CNI configuration files are
	// stored
	CNIConfigDir string `toml:"cni_config_dir"`
	// CNIPluginDir sets a number of directories where the CNI network
	// plugins can be located
	CNIPluginDir []string `toml:"cni_plugin_dir"`
	// CNIDefaultNetwork is the network name of the default CNI network
	// to attach pods to
	CNIDefaultNetwork string `toml:"cni_default_network,omitempty"`
	// HooksDir holds paths to the directories containing hooks
	// configuration files. When the same filename is present in in
	// multiple directories, the file in the directory listed last in
	// this slice takes precedence.
	HooksDir []string `toml:"hooks_dir"`
	// DefaultMountsFile is the path to the default mounts file for testing
	// purposes only
	DefaultMountsFile string `toml:"-"`
	// Namespace is the libpod namespace to use.
	// Namespaces are used to create scopes to separate containers and pods
	// in the state.
	// When namespace is set, libpod will only view containers and pods in
	// the same namespace. All containers and pods created will default to
	// the namespace set here.
	// A namespace of "", the empty string, is equivalent to no namespace,
	// and all containers and pods will be visible.
	// The default namespace is "".
	Namespace string `toml:"namespace,omitempty"`

	// InfraImage is the image a pod infra container will use to manage namespaces
	InfraImage string `toml:"infra_image"`
	// InfraCommand is the command run to start up a pod infra container
	InfraCommand string `toml:"infra_command"`
	// EnablePortReservation determines whether libpod will reserve ports on
	// the host when they are forwarded to containers.
	// When enabled, when ports are forwarded to containers, they are
	// held open by conmon as long as the container is running, ensuring
	// that they cannot be reused by other programs on the host.
	// However, this can cause significant memory usage if a container has
	// many ports forwarded to it. Disabling this can save memory.
	EnablePortReservation bool `toml:"enable_port_reservation"`
	// EnableLabeling indicates wether libpod will support container labeling
	EnableLabeling bool `toml:"label"`
	// NetworkCmdPath is the path to the slirp4netns binary
	NetworkCmdPath string `toml:"network_cmd_path"`

	// NumLocks is the number of locks to make available for containers and
	// pods.
	NumLocks uint32 `toml:"num_locks,omitempty"`

	// LockType is the type of locking to use.
	LockType string `toml:"lock_type,omitempty"`

	// EventsLogger determines where events should be logged
	EventsLogger string `toml:"events_logger"`
	// EventsLogFilePath is where the events log is stored.
	EventsLogFilePath string `toml:"events_logfile_path"`
	//DetachKeys is the sequence of keys used to detach a container
	DetachKeys string `toml:"detach_keys"`

	// SDNotify tells Libpod to allow containers to notify the host
	// systemd of readiness using the SD_NOTIFY mechanism
	SDNotify bool
	// CgroupCheck verifies if the cgroup check for correct OCI runtime has been done.
	CgroupCheck bool `toml:"cgroup_check,omitempty"`
}

// runtimeConfiguredFrom is a struct used during early runtime init to help
// assemble the full RuntimeConfig struct from defaults.
// It indicated whether several fields in the runtime configuration were set
// explicitly.
// If they were not, we may override them with information from the database,
// if it exists and differs from what is present in the system already.
type runtimeConfiguredFrom struct {
	storageGraphDriverSet    bool
	storageGraphRootSet      bool
	storageRunRootSet        bool
	libpodStaticDirSet       bool
	libpodTmpDirSet          bool
	volPathSet               bool
	conmonPath               bool
	conmonEnvVars            bool
	initPath                 bool
	ociRuntimes              bool
	runtimePath              bool
	cniPluginDir             bool
	noPivotRoot              bool
	runtimeSupportsJSON      bool
	runtimeSupportsNoCgroups bool
	ociRuntime               bool
}

func defaultRuntimeConfig() (RuntimeConfig, error) {
	storeOpts, err := storage.DefaultStoreOptions(rootless.IsRootless(), rootless.GetRootlessUID())
	if err != nil {
		return RuntimeConfig{}, err
	}
	graphRoot := storeOpts.GraphRoot
	if graphRoot == "" {
		logrus.Warnf("Storage configuration is unset - using hardcoded default paths")
		graphRoot = "/var/lib/containers/storage"
	}
	volumePath := filepath.Join(graphRoot, "volumes")
	staticDir := filepath.Join(graphRoot, "libpod")
	return RuntimeConfig{
		// Leave this empty so containers/storage will use its defaults
		StorageConfig:         storage.StoreOptions{},
		VolumePath:            volumePath,
		ImageDefaultTransport: DefaultTransport,
		StateType:             BoltDBStateStore,
		OCIRuntime:            "runc",
		OCIRuntimes: map[string][]string{
			"runc": {
				"/usr/bin/runc",
				"/usr/sbin/runc",
				"/usr/local/bin/runc",
				"/usr/local/sbin/runc",
				"/sbin/runc",
				"/bin/runc",
				"/usr/lib/cri-o-runc/sbin/runc",
				"/run/current-system/sw/bin/runc",
			},
		},
		ConmonPath: []string{
			"/usr/libexec/podman/conmon",
			"/usr/local/lib/podman/conmon",
			"/usr/bin/conmon",
			"/usr/sbin/conmon",
			"/usr/local/bin/conmon",
			"/usr/local/sbin/conmon",
			"/run/current-system/sw/bin/conmon",
		},
		ConmonEnvVars: []string{
			"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		},
		InitPath:              define.DefaultInitPath,
		CgroupManager:         SystemdCgroupsManager,
		StaticDir:             staticDir,
		TmpDir:                "",
		MaxLogSize:            -1,
		NoPivotRoot:           false,
		CNIConfigDir:          etcDir + "/cni/net.d/",
		CNIPluginDir:          []string{"/usr/libexec/cni", "/usr/lib/cni", "/usr/local/lib/cni", "/opt/cni/bin"},
		InfraCommand:          define.DefaultInfraCommand,
		InfraImage:            define.DefaultInfraImage,
		EnablePortReservation: true,
		EnableLabeling:        true,
		NumLocks:              2048,
		EventsLogger:          events.DefaultEventerType.String(),
		DetachKeys:            DefaultDetachKeys,
		LockType:              "shm",
	}, nil
}

// SetXdgDirs ensures the XDG_RUNTIME_DIR env and XDG_CONFIG_HOME variables are set.
// containers/image uses XDG_RUNTIME_DIR to locate the auth file, XDG_CONFIG_HOME is
// use for the libpod.conf configuration file.
// SetXdgDirs internally calls EnableLinger() so that the user's processes are not
// killed once the session is terminated.  EnableLinger() also attempts to
// get the runtime directory when XDG_RUNTIME_DIR is not specified.
// This function should only be called when running rootless.
func SetXdgDirs() error {
	if !rootless.IsRootless() {
		return nil
	}

	// Setup XDG_RUNTIME_DIR
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")

	runtimeDirLinger, err := rootless.EnableLinger()
	if err != nil {
		return errors.Wrapf(err, "error enabling user session")
	}
	if runtimeDir == "" && runtimeDirLinger != "" {
		if _, err := os.Stat(runtimeDirLinger); err != nil && os.IsNotExist(err) {
			chWait := make(chan error)
			defer close(chWait)
			if _, err := WaitForFile(runtimeDirLinger, chWait, time.Second*10); err != nil {
				return errors.Wrapf(err, "waiting for directory '%s'", runtimeDirLinger)
			}
		}
		runtimeDir = runtimeDirLinger
	}

	if runtimeDir == "" {
		var err error
		runtimeDir, err = util.GetRuntimeDir()
		if err != nil {
			return err
		}
	}
	if err := os.Setenv("XDG_RUNTIME_DIR", runtimeDir); err != nil {
		return errors.Wrapf(err, "cannot set XDG_RUNTIME_DIR")
	}

	if rootless.IsRootless() && os.Getenv("DBUS_SESSION_BUS_ADDRESS") == "" {
		sessionAddr := filepath.Join(runtimeDir, "bus")
		if _, err := os.Stat(sessionAddr); err == nil {
			os.Setenv("DBUS_SESSION_BUS_ADDRESS", fmt.Sprintf("unix:path=%s", sessionAddr))
		}
	}

	// Setup XDG_CONFIG_HOME
	if cfgHomeDir := os.Getenv("XDG_CONFIG_HOME"); cfgHomeDir == "" {
		if cfgHomeDir, err = util.GetRootlessConfigHomeDir(); err != nil {
			return err
		}
		if err = os.Setenv("XDG_CONFIG_HOME", cfgHomeDir); err != nil {
			return errors.Wrapf(err, "cannot set XDG_CONFIG_HOME")
		}
	}
	return nil
}

func getDefaultTmpDir() (string, error) {
	if !rootless.IsRootless() {
		return "/var/run/libpod", nil
	}

	runtimeDir, err := util.GetRuntimeDir()
	if err != nil {
		return "", err
	}
	libpodRuntimeDir := filepath.Join(runtimeDir, "libpod")

	if err := os.Mkdir(libpodRuntimeDir, 0700|os.ModeSticky); err != nil {
		if !os.IsExist(err) {
			return "", errors.Wrapf(err, "cannot mkdir %s", libpodRuntimeDir)
		} else if err := os.Chmod(libpodRuntimeDir, 0700|os.ModeSticky); err != nil {
			// The directory already exist, just set the sticky bit
			return "", errors.Wrapf(err, "could not set sticky bit on %s", libpodRuntimeDir)
		}
	}
	return filepath.Join(libpodRuntimeDir, "tmp"), nil
}

// NewRuntime creates a new container runtime
// Options can be passed to override the default configuration for the runtime
func NewRuntime(ctx context.Context, options ...RuntimeOption) (runtime *Runtime, err error) {
	return newRuntimeFromConfig(ctx, "", options...)
}

// NewRuntimeFromConfig creates a new container runtime using the given
// configuration file for its default configuration. Passed RuntimeOption
// functions can be used to mutate this configuration further.
// An error will be returned if the configuration file at the given path does
// not exist or cannot be loaded
func NewRuntimeFromConfig(ctx context.Context, userConfigPath string, options ...RuntimeOption) (runtime *Runtime, err error) {
	if userConfigPath == "" {
		return nil, errors.New("invalid configuration file specified")
	}
	return newRuntimeFromConfig(ctx, userConfigPath, options...)
}

func homeDir() (string, error) {
	home := os.Getenv("HOME")
	if home == "" {
		usr, err := user.LookupId(fmt.Sprintf("%d", rootless.GetRootlessUID()))
		if err != nil {
			return "", errors.Wrapf(err, "unable to resolve HOME directory")
		}
		home = usr.HomeDir
	}
	return home, nil
}

func getRootlessConfigPath() (string, error) {
	home, err := homeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, ".config/containers/libpod.conf"), nil
}

func getConfigPath() (string, error) {
	if rootless.IsRootless() {
		path, err := getRootlessConfigPath()
		if err != nil {
			return "", err
		}
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
		return "", err
	}
	if _, err := os.Stat(OverrideConfigPath); err == nil {
		// Use the override configuration path
		return OverrideConfigPath, nil
	}
	if _, err := os.Stat(ConfigPath); err == nil {
		return ConfigPath, nil
	}
	return "", nil
}

// DefaultRuntimeConfig reads default config path and returns the RuntimeConfig
func DefaultRuntimeConfig() (*RuntimeConfig, error) {
	configPath, err := getConfigPath()
	if err != nil {
		return nil, err
	}

	contents, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, errors.Wrapf(err, "error reading configuration file %s", configPath)
	}

	// This is ugly, but we need to decode twice.
	// Once to check if libpod static and tmp dirs were explicitly
	// set (not enough to check if they're not the default value,
	// might have been explicitly configured to the default).
	// A second time to actually get a usable config.
	tmpConfig := new(RuntimeConfig)
	if _, err := toml.Decode(string(contents), tmpConfig); err != nil {
		return nil, errors.Wrapf(err, "error decoding configuration file %s",
			configPath)
	}
	return tmpConfig, nil
}

func newRuntimeFromConfig(ctx context.Context, userConfigPath string, options ...RuntimeOption) (runtime *Runtime, err error) {
	runtime = new(Runtime)
	runtime.config = new(RuntimeConfig)
	runtime.configuredFrom = new(runtimeConfiguredFrom)

	// Copy the default configuration
	tmpDir, err := getDefaultTmpDir()
	if err != nil {
		return nil, err
	}

	defRunConf, err := defaultRuntimeConfig()
	if err != nil {
		return nil, err
	}
	if err := JSONDeepCopy(defRunConf, runtime.config); err != nil {
		return nil, errors.Wrapf(err, "error copying runtime default config")
	}
	runtime.config.TmpDir = tmpDir

	storageConf, err := storage.DefaultStoreOptions(rootless.IsRootless(), rootless.GetRootlessUID())
	if err != nil {
		return nil, errors.Wrapf(err, "error retrieving storage config")
	}
	runtime.config.StorageConfig = storageConf
	runtime.config.StaticDir = filepath.Join(storageConf.GraphRoot, "libpod")
	runtime.config.VolumePath = filepath.Join(storageConf.GraphRoot, "volumes")

	configPath, err := getConfigPath()
	if err != nil {
		return nil, err
	}
	if rootless.IsRootless() {
		home, err := homeDir()
		if err != nil {
			return nil, err
		}
		if runtime.config.SignaturePolicyPath == "" {
			newPath := filepath.Join(home, ".config/containers/policy.json")
			if _, err := os.Stat(newPath); err == nil {
				runtime.config.SignaturePolicyPath = newPath
			}
		}
	}

	if userConfigPath != "" {
		configPath = userConfigPath
		if _, err := os.Stat(configPath); err != nil {
			// If the user specified a config file, we must fail immediately
			// when it doesn't exist
			return nil, errors.Wrapf(err, "cannot stat %s", configPath)
		}
	}

	// If we have a valid configuration file, load it in
	if configPath != "" {
		contents, err := ioutil.ReadFile(configPath)
		if err != nil {
			return nil, errors.Wrapf(err, "error reading configuration file %s", configPath)
		}

		// This is ugly, but we need to decode twice.
		// Once to check if libpod static and tmp dirs were explicitly
		// set (not enough to check if they're not the default value,
		// might have been explicitly configured to the default).
		// A second time to actually get a usable config.
		tmpConfig := new(RuntimeConfig)
		if _, err := toml.Decode(string(contents), tmpConfig); err != nil {
			return nil, errors.Wrapf(err, "error decoding configuration file %s",
				configPath)
		}

		if err := cgroupV2Check(configPath, tmpConfig); err != nil {
			return nil, err
		}

		if tmpConfig.StaticDir != "" {
			runtime.configuredFrom.libpodStaticDirSet = true
		}
		if tmpConfig.TmpDir != "" {
			runtime.configuredFrom.libpodTmpDirSet = true
		}
		if tmpConfig.VolumePath != "" {
			runtime.configuredFrom.volPathSet = true
		}
		if tmpConfig.ConmonPath != nil {
			runtime.configuredFrom.conmonPath = true
		}
		if tmpConfig.ConmonEnvVars != nil {
			runtime.configuredFrom.conmonEnvVars = true
		}
		if tmpConfig.InitPath != "" {
			runtime.configuredFrom.initPath = true
		}
		if tmpConfig.OCIRuntimes != nil {
			runtime.configuredFrom.ociRuntimes = true
		}
		if tmpConfig.RuntimePath != nil {
			runtime.configuredFrom.runtimePath = true
		}
		if tmpConfig.CNIPluginDir != nil {
			runtime.configuredFrom.cniPluginDir = true
		}
		if tmpConfig.NoPivotRoot {
			runtime.configuredFrom.noPivotRoot = true
		}
		if tmpConfig.RuntimeSupportsJSON != nil {
			runtime.configuredFrom.runtimeSupportsJSON = true
		}
		if tmpConfig.RuntimeSupportsNoCgroups != nil {
			runtime.configuredFrom.runtimeSupportsNoCgroups = true
		}
		if tmpConfig.OCIRuntime != "" {
			runtime.configuredFrom.ociRuntime = true
		}

		if _, err := toml.Decode(string(contents), runtime.config); err != nil {
			return nil, errors.Wrapf(err, "error decoding configuration file %s", configPath)
		}
	} else if rootless.IsRootless() {
		// If the configuration file was not found but we are running in rootless, a subset of the
		// global config file is used.
		for _, path := range []string{OverrideConfigPath, ConfigPath} {
			contents, err := ioutil.ReadFile(path)
			if err != nil {
				// Ignore any error, the file might not be readable by us.
				continue
			}
			tmpConfig := new(RuntimeConfig)
			if _, err := toml.Decode(string(contents), tmpConfig); err != nil {
				return nil, errors.Wrapf(err, "error decoding configuration file %s", path)
			}

			// Cherry pick the settings we want from the global configuration
			if !runtime.configuredFrom.conmonPath {
				runtime.config.ConmonPath = tmpConfig.ConmonPath
			}
			if !runtime.configuredFrom.conmonEnvVars {
				runtime.config.ConmonEnvVars = tmpConfig.ConmonEnvVars
			}
			if !runtime.configuredFrom.initPath {
				runtime.config.InitPath = tmpConfig.InitPath
			}
			if !runtime.configuredFrom.ociRuntimes {
				runtime.config.OCIRuntimes = tmpConfig.OCIRuntimes
			}
			if !runtime.configuredFrom.runtimePath {
				runtime.config.RuntimePath = tmpConfig.RuntimePath
			}
			if !runtime.configuredFrom.cniPluginDir {
				runtime.config.CNIPluginDir = tmpConfig.CNIPluginDir
			}
			if !runtime.configuredFrom.noPivotRoot {
				runtime.config.NoPivotRoot = tmpConfig.NoPivotRoot
			}
			if !runtime.configuredFrom.runtimeSupportsJSON {
				runtime.config.RuntimeSupportsJSON = tmpConfig.RuntimeSupportsJSON
			}
			if !runtime.configuredFrom.runtimeSupportsNoCgroups {
				runtime.config.RuntimeSupportsNoCgroups = tmpConfig.RuntimeSupportsNoCgroups
			}
			if !runtime.configuredFrom.ociRuntime {
				runtime.config.OCIRuntime = tmpConfig.OCIRuntime
			}

			cgroupsV2, err := cgroups.IsCgroup2UnifiedMode()
			if err != nil {
				return nil, err
			}
			if cgroupsV2 {
				runtime.config.CgroupCheck = true
			}

			break
		}
	}

	// Overwrite config with user-given configuration options
	for _, opt := range options {
		if err := opt(runtime); err != nil {
			return nil, errors.Wrapf(err, "error configuring runtime")
		}
	}

	if rootless.IsRootless() && configPath == "" {
		configPath, err := getRootlessConfigPath()
		if err != nil {
			return nil, err
		}

		// storage.conf
		storageConfFile, err := storage.DefaultConfigFile(rootless.IsRootless())
		if err != nil {
			return nil, err
		}
		if _, err := os.Stat(storageConfFile); os.IsNotExist(err) {
			if err := util.WriteStorageConfigFile(&runtime.config.StorageConfig, storageConfFile); err != nil {
				return nil, errors.Wrapf(err, "cannot write config file %s", storageConfFile)
			}
		}

		if configPath != "" {
			if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
				return nil, err
			}
			file, err := os.OpenFile(configPath, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0666)
			if err != nil && !os.IsExist(err) {
				return nil, errors.Wrapf(err, "cannot open file %s", configPath)
			}
			if err == nil {
				defer file.Close()
				enc := toml.NewEncoder(file)
				if err := enc.Encode(runtime.config); err != nil {
					if removeErr := os.Remove(configPath); removeErr != nil {
						logrus.Debugf("unable to remove %s: %q", configPath, err)
					}
				}
			}
		}
	}
	if err := makeRuntime(ctx, runtime); err != nil {
		return nil, err
	}
	return runtime, nil
}

func getLockManager(runtime *Runtime) (lock.Manager, error) {
	var err error
	var manager lock.Manager

	switch runtime.config.LockType {
	case "file":
		lockPath := filepath.Join(runtime.config.TmpDir, "locks")
		manager, err = lock.OpenFileLockManager(lockPath)
		if err != nil {
			if os.IsNotExist(errors.Cause(err)) {
				manager, err = lock.NewFileLockManager(lockPath)
				if err != nil {
					return nil, errors.Wrapf(err, "failed to get new file lock manager")
				}
			} else {
				return nil, err
			}
		}

	case "", "shm":
		lockPath := DefaultSHMLockPath
		if rootless.IsRootless() {
			lockPath = fmt.Sprintf("%s_%d", DefaultRootlessSHMLockPath, rootless.GetRootlessUID())
		}
		// Set up the lock manager
		manager, err = lock.OpenSHMLockManager(lockPath, runtime.config.NumLocks)
		if err != nil {
			if os.IsNotExist(errors.Cause(err)) {
				manager, err = lock.NewSHMLockManager(lockPath, runtime.config.NumLocks)
				if err != nil {
					return nil, errors.Wrapf(err, "failed to get new shm lock manager")
				}
			} else if errors.Cause(err) == syscall.ERANGE && runtime.doRenumber {
				logrus.Debugf("Number of locks does not match - removing old locks")

				// ERANGE indicates a lock numbering mismatch.
				// Since we're renumbering, this is not fatal.
				// Remove the earlier set of locks and recreate.
				if err := os.Remove(filepath.Join("/dev/shm", lockPath)); err != nil {
					return nil, errors.Wrapf(err, "error removing libpod locks file %s", lockPath)
				}

				manager, err = lock.NewSHMLockManager(lockPath, runtime.config.NumLocks)
				if err != nil {
					return nil, err
				}
			} else {
				return nil, err
			}
		}
	default:
		return nil, errors.Wrapf(define.ErrInvalidArg, "unknown lock type %s", runtime.config.LockType)
	}
	return manager, nil
}

// probeConmon calls conmon --version and verifies it is a new enough version for
// the runtime expectations podman currently has
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
		return errors.Wrapf(err, "conmon version changed format")
	}
	major, err := strconv.Atoi(matches[1])
	if err != nil || major < 1 {
		return define.ErrConmonOutdated
	}
	// conmon used to be shipped with CRI-O, and was versioned along with it.
	// even though the conmon that came with crio-1.9 to crio-1.15 has a higher
	// version number than conmon 1.0.0, 1.0.0 is newer, so we need this check
	minor, err := strconv.Atoi(matches[2])
	if err != nil || minor > 9 {
		return define.ErrConmonOutdated
	}

	return nil
}

// Make a new runtime based on the given configuration
// Sets up containers/storage, state store, OCI runtime
func makeRuntime(ctx context.Context, runtime *Runtime) (err error) {
	// Let's sanity-check some paths first.
	// Relative paths can cause nasty bugs, because core paths we use could
	// shift between runs (or even parts of the program - the OCI runtime
	// uses a different working directory than we do, for example.
	if !filepath.IsAbs(runtime.config.StaticDir) {
		return errors.Wrapf(define.ErrInvalidArg, "static directory must be an absolute path - instead got %q", runtime.config.StaticDir)
	}
	if !filepath.IsAbs(runtime.config.TmpDir) {
		return errors.Wrapf(define.ErrInvalidArg, "temporary directory must be an absolute path - instead got %q", runtime.config.TmpDir)
	}
	if !filepath.IsAbs(runtime.config.VolumePath) {
		return errors.Wrapf(define.ErrInvalidArg, "volume path must be an absolute path - instead got %q", runtime.config.VolumePath)
	}

	// Find a working conmon binary
	foundConmon := false
	foundOutdatedConmon := false
	for _, path := range runtime.config.ConmonPath {
		stat, err := os.Stat(path)
		if err != nil {
			continue
		}
		if stat.IsDir() {
			continue
		}
		if err := probeConmon(path); err != nil {
			logrus.Warnf("conmon at %s invalid: %v", path, err)
			foundOutdatedConmon = true
			continue
		}
		foundConmon = true
		runtime.conmonPath = path
		logrus.Debugf("using conmon: %q", path)
		break
	}

	// Search the $PATH as last fallback
	if !foundConmon {
		if conmon, err := exec.LookPath("conmon"); err == nil {
			if err := probeConmon(conmon); err != nil {
				logrus.Warnf("conmon at %s is invalid: %v", conmon, err)
				foundOutdatedConmon = true
			} else {
				foundConmon = true
				runtime.conmonPath = conmon
				logrus.Debugf("using conmon from $PATH: %q", conmon)
			}
		}
	}

	if !foundConmon {
		if foundOutdatedConmon {
			return errors.Wrapf(define.ErrConmonOutdated, "please update to v1.0.0 or later")
		}
		return errors.Wrapf(define.ErrInvalidArg,
			"could not find a working conmon binary (configured options: %v)",
			runtime.config.ConmonPath)
	}

	// Make the static files directory if it does not exist
	if err := os.MkdirAll(runtime.config.StaticDir, 0700); err != nil {
		// The directory is allowed to exist
		if !os.IsExist(err) {
			return errors.Wrapf(err, "error creating runtime static files directory %s",
				runtime.config.StaticDir)
		}
	}

	// Set up the state
	switch runtime.config.StateType {
	case InMemoryStateStore:
		state, err := NewInMemoryState()
		if err != nil {
			return err
		}
		runtime.state = state
	case SQLiteStateStore:
		return errors.Wrapf(define.ErrInvalidArg, "SQLite state is currently disabled")
	case BoltDBStateStore:
		dbPath := filepath.Join(runtime.config.StaticDir, "bolt_state.db")

		state, err := NewBoltState(dbPath, runtime)
		if err != nil {
			return err
		}
		runtime.state = state
	default:
		return errors.Wrapf(define.ErrInvalidArg, "unrecognized state type passed")
	}

	// Grab config from the database so we can reset some defaults
	dbConfig, err := runtime.state.GetDBConfig()
	if err != nil {
		return errors.Wrapf(err, "error retrieving runtime configuration from database")
	}

	// Reset defaults if they were not explicitly set
	if !runtime.configuredFrom.storageGraphDriverSet && dbConfig.GraphDriver != "" {
		if runtime.config.StorageConfig.GraphDriverName != dbConfig.GraphDriver &&
			runtime.config.StorageConfig.GraphDriverName != "" {
			logrus.Errorf("User-selected graph driver %q overwritten by graph driver %q from database - delete libpod local files to resolve",
				runtime.config.StorageConfig.GraphDriverName, dbConfig.GraphDriver)
		}
		runtime.config.StorageConfig.GraphDriverName = dbConfig.GraphDriver
	}
	if !runtime.configuredFrom.storageGraphRootSet && dbConfig.StorageRoot != "" {
		if runtime.config.StorageConfig.GraphRoot != dbConfig.StorageRoot &&
			runtime.config.StorageConfig.GraphRoot != "" {
			logrus.Debugf("Overriding graph root %q with %q from database",
				runtime.config.StorageConfig.GraphRoot, dbConfig.StorageRoot)
		}
		runtime.config.StorageConfig.GraphRoot = dbConfig.StorageRoot
	}
	if !runtime.configuredFrom.storageRunRootSet && dbConfig.StorageTmp != "" {
		if runtime.config.StorageConfig.RunRoot != dbConfig.StorageTmp &&
			runtime.config.StorageConfig.RunRoot != "" {
			logrus.Debugf("Overriding run root %q with %q from database",
				runtime.config.StorageConfig.RunRoot, dbConfig.StorageTmp)
		}
		runtime.config.StorageConfig.RunRoot = dbConfig.StorageTmp
	}
	if !runtime.configuredFrom.libpodStaticDirSet && dbConfig.LibpodRoot != "" {
		if runtime.config.StaticDir != dbConfig.LibpodRoot && runtime.config.StaticDir != "" {
			logrus.Debugf("Overriding static dir %q with %q from database", runtime.config.StaticDir, dbConfig.LibpodRoot)
		}
		runtime.config.StaticDir = dbConfig.LibpodRoot
	}
	if !runtime.configuredFrom.libpodTmpDirSet && dbConfig.LibpodTmp != "" {
		if runtime.config.TmpDir != dbConfig.LibpodTmp && runtime.config.TmpDir != "" {
			logrus.Debugf("Overriding tmp dir %q with %q from database", runtime.config.TmpDir, dbConfig.LibpodTmp)
		}
		runtime.config.TmpDir = dbConfig.LibpodTmp
	}
	if !runtime.configuredFrom.volPathSet && dbConfig.VolumePath != "" {
		if runtime.config.VolumePath != dbConfig.VolumePath && runtime.config.VolumePath != "" {
			logrus.Debugf("Overriding volume path %q with %q from database", runtime.config.VolumePath, dbConfig.VolumePath)
		}
		runtime.config.VolumePath = dbConfig.VolumePath
	}

	runtime.config.EventsLogFilePath = filepath.Join(runtime.config.TmpDir, "events", "events.log")

	logrus.Debugf("Using graph driver %s", runtime.config.StorageConfig.GraphDriverName)
	logrus.Debugf("Using graph root %s", runtime.config.StorageConfig.GraphRoot)
	logrus.Debugf("Using run root %s", runtime.config.StorageConfig.RunRoot)
	logrus.Debugf("Using static dir %s", runtime.config.StaticDir)
	logrus.Debugf("Using tmp dir %s", runtime.config.TmpDir)
	logrus.Debugf("Using volume path %s", runtime.config.VolumePath)

	// Validate our config against the database, now that we've set our
	// final storage configuration
	if err := runtime.state.ValidateDBConfig(runtime); err != nil {
		return err
	}

	if err := runtime.state.SetNamespace(runtime.config.Namespace); err != nil {
		return errors.Wrapf(err, "error setting libpod namespace in state")
	}
	logrus.Debugf("Set libpod namespace to %q", runtime.config.Namespace)

	// Set up containers/storage
	var store storage.Store
	if os.Geteuid() != 0 {
		logrus.Debug("Not configuring container store")
	} else if runtime.noStore {
		logrus.Debug("No store required. Not opening container store.")
	} else {
		if err := runtime.configureStore(); err != nil {
			return err
		}
	}
	defer func() {
		if err != nil && store != nil {
			// Don't forcibly shut down
			// We could be opening a store in use by another libpod
			_, err2 := store.Shutdown(false)
			if err2 != nil {
				logrus.Errorf("Error removing store for partially-created runtime: %s", err2)
			}
		}
	}()

	// Setup the eventer
	eventer, err := runtime.newEventer()
	if err != nil {
		return err
	}
	runtime.eventer = eventer
	if runtime.imageRuntime != nil {
		runtime.imageRuntime.Eventer = eventer
	}

	// Set up containers/image
	runtime.imageContext = &types.SystemContext{
		SignaturePolicyPath: runtime.config.SignaturePolicyPath,
	}

	// Create the tmpDir
	if err := os.MkdirAll(runtime.config.TmpDir, 0751); err != nil {
		// The directory is allowed to exist
		if !os.IsExist(err) {
			return errors.Wrapf(err, "error creating tmpdir %s", runtime.config.TmpDir)
		}
	}

	// Create events log dir
	if err := os.MkdirAll(filepath.Dir(runtime.config.EventsLogFilePath), 0700); err != nil {
		// The directory is allowed to exist
		if !os.IsExist(err) {
			return errors.Wrapf(err, "error creating events dirs %s", filepath.Dir(runtime.config.EventsLogFilePath))
		}
	}

	// Make lookup tables for runtime support
	supportsJSON := make(map[string]bool)
	supportsNoCgroups := make(map[string]bool)
	for _, r := range runtime.config.RuntimeSupportsJSON {
		supportsJSON[r] = true
	}
	for _, r := range runtime.config.RuntimeSupportsNoCgroups {
		supportsNoCgroups[r] = true
	}

	// Get us at least one working OCI runtime.
	runtime.ociRuntimes = make(map[string]OCIRuntime)

	// Is the old runtime_path defined?
	if runtime.config.RuntimePath != nil {
		// Don't print twice in rootless mode.
		if os.Geteuid() == 0 {
			logrus.Warningf("The configuration is using `runtime_path`, which is deprecated and will be removed in future.  Please use `runtimes` and `runtime`")
			logrus.Warningf("If you are using both `runtime_path` and `runtime`, the configuration from `runtime_path` is used")
		}

		if len(runtime.config.RuntimePath) == 0 {
			return errors.Wrapf(define.ErrInvalidArg, "empty runtime path array passed")
		}

		name := filepath.Base(runtime.config.RuntimePath[0])

		json := supportsJSON[name]
		nocgroups := supportsNoCgroups[name]

		ociRuntime, err := newConmonOCIRuntime(name, runtime.config.RuntimePath, runtime.conmonPath, runtime.config, json, nocgroups)
		if err != nil {
			return err
		}

		runtime.ociRuntimes[name] = ociRuntime
		runtime.defaultOCIRuntime = ociRuntime
	}

	// Initialize remaining OCI runtimes
	for name, paths := range runtime.config.OCIRuntimes {
		json := supportsJSON[name]
		nocgroups := supportsNoCgroups[name]

		ociRuntime, err := newConmonOCIRuntime(name, paths, runtime.conmonPath, runtime.config, json, nocgroups)
		if err != nil {
			// Don't fatally error.
			// This will allow us to ship configs including optional
			// runtimes that might not be installed (crun, kata).
			// Only a warnf so default configs don't spec errors.
			logrus.Warnf("Error initializing configured OCI runtime %s: %v", name, err)
			continue
		}

		runtime.ociRuntimes[name] = ociRuntime
	}

	// Do we have a default OCI runtime?
	if runtime.config.OCIRuntime != "" {
		// If the string starts with / it's a path to a runtime
		// executable.
		if strings.HasPrefix(runtime.config.OCIRuntime, "/") {
			name := filepath.Base(runtime.config.OCIRuntime)

			json := supportsJSON[name]
			nocgroups := supportsNoCgroups[name]

			ociRuntime, err := newConmonOCIRuntime(name, []string{runtime.config.OCIRuntime}, runtime.conmonPath, runtime.config, json, nocgroups)
			if err != nil {
				return err
			}

			runtime.ociRuntimes[name] = ociRuntime
			runtime.defaultOCIRuntime = ociRuntime
		} else {
			ociRuntime, ok := runtime.ociRuntimes[runtime.config.OCIRuntime]
			if !ok {
				return errors.Wrapf(define.ErrInvalidArg, "default OCI runtime %q not found", runtime.config.OCIRuntime)
			}
			runtime.defaultOCIRuntime = ociRuntime
		}
	}

	// Do we have at least one valid OCI runtime?
	if len(runtime.ociRuntimes) == 0 {
		return errors.Wrapf(define.ErrInvalidArg, "no OCI runtime has been configured")
	}

	// Do we have a default runtime?
	if runtime.defaultOCIRuntime == nil {
		return errors.Wrapf(define.ErrInvalidArg, "no default OCI runtime was configured")
	}

	// Make the per-boot files directory if it does not exist
	if err := os.MkdirAll(runtime.config.TmpDir, 0755); err != nil {
		// The directory is allowed to exist
		if !os.IsExist(err) {
			return errors.Wrapf(err, "error creating runtime temporary files directory %s",
				runtime.config.TmpDir)
		}
	}

	// Set up the CNI net plugin
	if !rootless.IsRootless() {
		netPlugin, err := ocicni.InitCNI(runtime.config.CNIDefaultNetwork, runtime.config.CNIConfigDir, runtime.config.CNIPluginDir...)
		if err != nil {
			return errors.Wrapf(err, "error configuring CNI network plugin")
		}
		runtime.netPlugin = netPlugin
	}

	// We now need to see if the system has restarted
	// We check for the presence of a file in our tmp directory to verify this
	// This check must be locked to prevent races
	runtimeAliveLock := filepath.Join(runtime.config.TmpDir, "alive.lck")
	runtimeAliveFile := filepath.Join(runtime.config.TmpDir, "alive")
	aliveLock, err := storage.GetLockfile(runtimeAliveLock)
	if err != nil {
		return errors.Wrapf(err, "error acquiring runtime init lock")
	}
	// Acquire the lock and hold it until we return
	// This ensures that no two processes will be in runtime.refresh at once
	// TODO: we can't close the FD in this lock, so we should keep it around
	// and use it to lock important operations
	aliveLock.Lock()
	doRefresh := false
	defer func() {
		if aliveLock.Locked() {
			aliveLock.Unlock()
		}
	}()

	_, err = os.Stat(runtimeAliveFile)
	if err != nil {
		// If we need to refresh, then it is safe to assume there are
		// no containers running.  Create immediately a namespace, as
		// we will need to access the storage.
		if os.Geteuid() != 0 {
			aliveLock.Unlock() // Unlock to avoid deadlock as BecomeRootInUserNS will reexec.
			pausePid, err := util.GetRootlessPauseProcessPidPath()
			if err != nil {
				return errors.Wrapf(err, "could not get pause process pid file path")
			}
			became, ret, err := rootless.BecomeRootInUserNS(pausePid)
			if err != nil {
				return err
			}
			if became {
				os.Exit(ret)
			}

		}
		// If the file doesn't exist, we need to refresh the state
		// This will trigger on first use as well, but refreshing an
		// empty state only creates a single file
		// As such, it's not really a performance concern
		if os.IsNotExist(err) {
			doRefresh = true
		} else {
			return errors.Wrapf(err, "error reading runtime status file %s", runtimeAliveFile)
		}
	}

	runtime.lockManager, err = getLockManager(runtime)
	if err != nil {
		return err
	}

	// If we're renumbering locks, do it now.
	// It breaks out of normal runtime init, and will not return a valid
	// runtime.
	if runtime.doRenumber {
		if err := runtime.renumberLocks(); err != nil {
			return err
		}
	}

	// If we need to refresh the state, do it now - things are guaranteed to
	// be set up by now.
	if doRefresh {
		// Ensure we have a store before refresh occurs
		if runtime.store == nil {
			if err := runtime.configureStore(); err != nil {
				return err
			}
		}

		if err2 := runtime.refresh(runtimeAliveFile); err2 != nil {
			return err2
		}
	}

	// Mark the runtime as valid - ready to be used, cannot be modified
	// further
	runtime.valid = true

	if runtime.doMigrate {
		if err := runtime.migrate(ctx); err != nil {
			return err
		}
	}

	return nil
}

// GetConfig returns a copy of the configuration used by the runtime
func (r *Runtime) GetConfig() (*RuntimeConfig, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	if !r.valid {
		return nil, define.ErrRuntimeStopped
	}

	config := new(RuntimeConfig)

	// Copy so the caller won't be able to modify the actual config
	if err := JSONDeepCopy(r.config, config); err != nil {
		return nil, errors.Wrapf(err, "error copying config")
	}

	return config, nil
}

// DeferredShutdown shuts down the runtime without exposing any
// errors. This is only meant to be used when the runtime is being
// shutdown within a defer statement; else use Shutdown
func (r *Runtime) DeferredShutdown(force bool) {
	_ = r.Shutdown(force)
}

// Shutdown shuts down the runtime and associated containers and storage
// If force is true, containers and mounted storage will be shut down before
// cleaning up; if force is false, an error will be returned if there are
// still containers running or mounted
func (r *Runtime) Shutdown(force bool) error {
	r.lock.Lock()
	defer r.lock.Unlock()

	if !r.valid {
		return define.ErrRuntimeStopped
	}

	r.valid = false

	// Shutdown all containers if --force is given
	if force {
		ctrs, err := r.state.AllContainers()
		if err != nil {
			logrus.Errorf("Error retrieving containers from database: %v", err)
		} else {
			for _, ctr := range ctrs {
				if err := ctr.StopWithTimeout(define.CtrRemoveTimeout); err != nil {
					logrus.Errorf("Error stopping container %s: %v", ctr.ID(), err)
				}
			}
		}
	}

	var lastError error
	// If no store was requested, it can bew nil and there is no need to
	// attempt to shut it down
	if r.store != nil {
		if _, err := r.store.Shutdown(force); err != nil {
			lastError = errors.Wrapf(err, "Error shutting down container storage")
		}
	}
	if err := r.state.Close(); err != nil {
		if lastError != nil {
			logrus.Errorf("%v", lastError)
		}
		lastError = err
	}

	return lastError
}

// Reconfigures the runtime after a reboot
// Refreshes the state, recreating temporary files
// Does not check validity as the runtime is not valid until after this has run
func (r *Runtime) refresh(alivePath string) error {
	logrus.Debugf("Podman detected system restart - performing state refresh")

	// First clear the state in the database
	if err := r.state.Refresh(); err != nil {
		return err
	}

	// Next refresh the state of all containers to recreate dirs and
	// namespaces, and all the pods to recreate cgroups
	ctrs, err := r.state.AllContainers()
	if err != nil {
		return errors.Wrapf(err, "error retrieving all containers from state")
	}
	pods, err := r.state.AllPods()
	if err != nil {
		return errors.Wrapf(err, "error retrieving all pods from state")
	}
	// No locks are taken during pod and container refresh.
	// Furthermore, the pod and container refresh() functions are not
	// allowed to take locks themselves.
	// We cannot assume that any pod or container has a valid lock until
	// after this function has returned.
	// The runtime alive lock should suffice to provide mutual exclusion
	// until this has run.
	for _, ctr := range ctrs {
		if err := ctr.refresh(); err != nil {
			logrus.Errorf("Error refreshing container %s: %v", ctr.ID(), err)
		}
	}
	for _, pod := range pods {
		if err := pod.refresh(); err != nil {
			logrus.Errorf("Error refreshing pod %s: %v", pod.ID(), err)
		}
	}

	// Create a file indicating the runtime is alive and ready
	file, err := os.OpenFile(alivePath, os.O_RDONLY|os.O_CREATE, 0644)
	if err != nil {
		return errors.Wrapf(err, "error creating runtime status file %s", alivePath)
	}
	defer file.Close()

	r.newSystemEvent(events.Refresh)

	return nil
}

// Info returns the store and host information
func (r *Runtime) Info() ([]define.InfoData, error) {
	info := []define.InfoData{}
	// get host information
	hostInfo, err := r.hostInfo()
	if err != nil {
		return nil, errors.Wrapf(err, "error getting host info")
	}
	info = append(info, define.InfoData{Type: "host", Data: hostInfo})

	// get store information
	storeInfo, err := r.storeInfo()
	if err != nil {
		return nil, errors.Wrapf(err, "error getting store info")
	}
	info = append(info, define.InfoData{Type: "store", Data: storeInfo})

	reg, err := sysreg.GetRegistries()
	if err != nil {
		return nil, errors.Wrapf(err, "error getting registries")
	}
	registries := make(map[string]interface{})
	registries["search"] = reg

	ireg, err := sysreg.GetInsecureRegistries()
	if err != nil {
		return nil, errors.Wrapf(err, "error getting registries")
	}
	registries["insecure"] = ireg

	breg, err := sysreg.GetBlockedRegistries()
	if err != nil {
		return nil, errors.Wrapf(err, "error getting registries")
	}
	registries["blocked"] = breg
	info = append(info, define.InfoData{Type: "registries", Data: registries})
	return info, nil
}

// generateName generates a unique name for a container or pod.
func (r *Runtime) generateName() (string, error) {
	for {
		name := namesgenerator.GetRandomName(0)
		// Make sure container with this name does not exist
		if _, err := r.state.LookupContainer(name); err == nil {
			continue
		} else {
			if errors.Cause(err) != define.ErrNoSuchCtr {
				return "", err
			}
		}
		// Make sure pod with this name does not exist
		if _, err := r.state.LookupPod(name); err == nil {
			continue
		} else {
			if errors.Cause(err) != define.ErrNoSuchPod {
				return "", err
			}
		}
		return name, nil
	}
	// The code should never reach here.
}

// Configure store and image runtime
func (r *Runtime) configureStore() error {
	store, err := storage.GetStore(r.config.StorageConfig)
	if err != nil {
		return err
	}

	r.store = store
	is.Transport.SetStore(store)

	// Set up a storage service for creating container root filesystems from
	// images
	storageService, err := getStorageService(r.store)
	if err != nil {
		return err
	}
	r.storageService = storageService

	ir := image.NewImageRuntimeFromStore(r.store)
	ir.SignaturePolicyPath = r.config.SignaturePolicyPath
	ir.EventsLogFilePath = r.config.EventsLogFilePath
	ir.EventsLogger = r.config.EventsLogger

	r.imageRuntime = ir

	return nil
}

// ImageRuntime returns the imageruntime for image operations.
// If WithNoStore() was used, no image runtime will be available, and this
// function will return nil.
func (r *Runtime) ImageRuntime() *image.Runtime {
	return r.imageRuntime
}

// SystemContext returns the imagecontext
func (r *Runtime) SystemContext() *types.SystemContext {
	return r.imageContext
}

// GetOCIRuntimePath retrieves the path of the default OCI runtime.
func (r *Runtime) GetOCIRuntimePath() string {
	return r.defaultOCIRuntime.Path()
}

// Since runc does not currently support cgroupV2
// Change to default crun on first running of libpod.conf
// TODO Once runc has support for cgroups, this function should be removed.
func cgroupV2Check(configPath string, tmpConfig *RuntimeConfig) error {
	if !tmpConfig.CgroupCheck && rootless.IsRootless() {
		cgroupsV2, err := cgroups.IsCgroup2UnifiedMode()
		if err != nil {
			return err
		}
		if cgroupsV2 {
			path, err := exec.LookPath("crun")
			if err != nil {
				logrus.Warnf("Can not find crun package on the host, containers might fail to run on cgroup V2 systems without crun: %q", err)
				// Can't find crun path so do nothing
				return nil
			}
			tmpConfig.CgroupCheck = true
			tmpConfig.OCIRuntime = path
			file, err := os.OpenFile(configPath, os.O_RDWR|os.O_CREATE, 0666)
			if err != nil {
				return errors.Wrapf(err, "cannot open file %s", configPath)
			}
			defer file.Close()
			enc := toml.NewEncoder(file)
			if err := enc.Encode(tmpConfig); err != nil {
				if removeErr := os.Remove(configPath); removeErr != nil {
					logrus.Debugf("unable to remove %s: %q", configPath, err)
				}
			}
		}
	}
	return nil
}
