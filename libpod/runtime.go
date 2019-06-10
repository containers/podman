package libpod

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/BurntSushi/toml"
	is "github.com/containers/image/storage"
	"github.com/containers/image/types"
	"github.com/containers/libpod/libpod/events"
	"github.com/containers/libpod/libpod/image"
	"github.com/containers/libpod/libpod/lock"
	"github.com/containers/libpod/pkg/firewall"
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
	installPrefix = "/usr/local"
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

	// DefaultInfraImage to use for infra container
	DefaultInfraImage = "k8s.gcr.io/pause:3.1"
	// DefaultInfraCommand to be run in an infra container
	DefaultInfraCommand = "/pause"

	// DefaultInitPath is the default path to the container-init binary
	DefaultInitPath = "/usr/libexec/podman/catatonit"

	// DefaultSHMLockPath is the default path for SHM locks
	DefaultSHMLockPath = "/libpod_lock"
	// DefaultRootlessSHMLockPath is the default path for rootless SHM locks
	DefaultRootlessSHMLockPath = "/libpod_rootless_lock"
)

// A RuntimeOption is a functional option which alters the Runtime created by
// NewRuntime
type RuntimeOption func(*Runtime) error

// Runtime is the core libpod runtime
type Runtime struct {
	config *RuntimeConfig

	state           State
	store           storage.Store
	storageService  *storageService
	imageContext    *types.SystemContext
	ociRuntime      *OCIRuntime
	netPlugin       ocicni.CNIPlugin
	ociRuntimePath  OCIRuntimePath
	conmonPath      string
	imageRuntime    *image.Runtime
	firewallBackend firewall.FirewallBackend
	lockManager     lock.Manager
	configuredFrom  *runtimeConfiguredFrom

	// doRenumber indicates that the runtime should perform a lock renumber
	// during initialization.
	// Once the runtime has been initialized and returned, this variable is
	// unused.
	doRenumber bool

	doMigrate bool

	// valid indicates whether the runtime is ready to use.
	// valid is set to true when a runtime is returned from GetRuntime(),
	// and remains true until the runtime is shut down (rendering its
	// storage unusable). When valid is false, the runtime cannot be used.
	valid bool
	lock  sync.RWMutex

	// mechanism to read and write even logs
	eventer events.Eventer
}

// OCIRuntimePath contains information about an OCI runtime.
type OCIRuntimePath struct {
	// Name of the runtime to refer to by the --runtime flag
	Name string `toml:"name"`
	// Paths to check for this executable
	Paths []string `toml:"paths"`
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
	// RuntimeSupportsJSON is the list of the OCI runtimes that support --format=json
	RuntimeSupportsJSON []string `toml:"runtime_supports_json"`
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

	// EventsLogger determines where events should be logged
	EventsLogger string `toml:"events_logger"`
	// EventsLogFilePath is where the events log is stored.
	EventsLogFilePath string `toml:-"events_logfile_path"`
}

// runtimeConfiguredFrom is a struct used during early runtime init to help
// assemble the full RuntimeConfig struct from defaults.
// It indicated whether several fields in the runtime configuration were set
// explicitly.
// If they were not, we may override them with information from the database,
// if it exists and differs from what is present in the system already.
type runtimeConfiguredFrom struct {
	storageGraphDriverSet bool
	storageGraphRootSet   bool
	storageRunRootSet     bool
	libpodStaticDirSet    bool
	libpodTmpDirSet       bool
	volPathSet            bool
	conmonPath            bool
	conmonEnvVars         bool
	initPath              bool
	ociRuntimes           bool
	runtimePath           bool
	cniPluginDir          bool
	noPivotRoot           bool
}

func defaultRuntimeConfig() (RuntimeConfig, error) {
	storeOpts, err := storage.DefaultStoreOptions(rootless.IsRootless(), rootless.GetRootlessUID())
	if err != nil {
		return RuntimeConfig{}, err
	}
	return RuntimeConfig{
		// Leave this empty so containers/storage will use its defaults
		StorageConfig:         storage.StoreOptions{},
		VolumePath:            filepath.Join(storeOpts.GraphRoot, "volumes"),
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
			},
		},
		ConmonPath: []string{
			"/usr/libexec/podman/conmon",
			"/usr/libexec/crio/conmon",
			"/usr/local/lib/podman/conmon",
			"/usr/local/libexec/crio/conmon",
			"/usr/bin/conmon",
			"/usr/sbin/conmon",
			"/usr/lib/crio/bin/conmon",
		},
		ConmonEnvVars: []string{
			"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		},
		InitPath:              DefaultInitPath,
		CgroupManager:         SystemdCgroupsManager,
		StaticDir:             filepath.Join(storeOpts.GraphRoot, "libpod"),
		TmpDir:                "",
		MaxLogSize:            -1,
		NoPivotRoot:           false,
		CNIConfigDir:          etcDir + "/cni/net.d/",
		CNIPluginDir:          []string{"/usr/libexec/cni", "/usr/lib/cni", "/usr/local/lib/cni", "/opt/cni/bin"},
		InfraCommand:          DefaultInfraCommand,
		InfraImage:            DefaultInfraImage,
		EnablePortReservation: true,
		EnableLabeling:        true,
		NumLocks:              2048,
		EventsLogger:          events.DefaultEventerType.String(),
	}, nil
}

func getDefaultTmpDir() (string, error) {
	if !rootless.IsRootless() {
		return "/var/run/libpod", nil
	}

	rootlessRuntimeDir, err := util.GetRootlessRuntimeDir()
	if err != nil {
		return "", err
	}
	libpodRuntimeDir := filepath.Join(rootlessRuntimeDir, "libpod")

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

// SetXdgRuntimeDir ensures the XDG_RUNTIME_DIR env variable is set
// containers/image uses XDG_RUNTIME_DIR to locate the auth file.
func SetXdgRuntimeDir(val string) error {
	if !rootless.IsRootless() {
		return nil
	}
	if val == "" {
		var err error
		val, err = util.GetRootlessRuntimeDir()
		if err != nil {
			return err
		}
	}
	if err := os.Setenv("XDG_RUNTIME_DIR", val); err != nil {
		return errors.Wrapf(err, "cannot set XDG_RUNTIME_DIR")
	}
	return nil
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

	configPath := ConfigPath
	foundConfig := true
	rootlessConfigPath := ""
	if rootless.IsRootless() {
		home := os.Getenv("HOME")
		if runtime.config.SignaturePolicyPath == "" {
			newPath := filepath.Join(home, ".config/containers/policy.json")
			if _, err := os.Stat(newPath); err == nil {
				runtime.config.SignaturePolicyPath = newPath
			}
		}

		rootlessConfigPath = filepath.Join(home, ".config/containers/libpod.conf")

		runtimeDir, err := util.GetRootlessRuntimeDir()
		if err != nil {
			return nil, err
		}

		// containers/image uses XDG_RUNTIME_DIR to locate the auth file.
		// So make sure the env variable is set.
		if err := SetXdgRuntimeDir(runtimeDir); err != nil {
			return nil, errors.Wrapf(err, "cannot set XDG_RUNTIME_DIR")
		}

	}

	if userConfigPath != "" {
		configPath = userConfigPath
		if _, err := os.Stat(configPath); err != nil {
			// If the user specified a config file, we must fail immediately
			// when it doesn't exist
			return nil, errors.Wrapf(err, "cannot stat %s", configPath)
		}
	} else if rootless.IsRootless() {
		configPath = rootlessConfigPath
		if _, err := os.Stat(configPath); err != nil {
			foundConfig = false
		}
	} else if _, err := os.Stat(OverrideConfigPath); err == nil {
		// Use the override configuration path
		configPath = OverrideConfigPath
	} else if _, err := os.Stat(ConfigPath); err != nil {
		// Both stat checks failed, no config found
		foundConfig = false
	}

	// If we have a valid configuration file, load it in
	if foundConfig {
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
			break
		}
	}

	// Overwrite config with user-given configuration options
	for _, opt := range options {
		if err := opt(runtime); err != nil {
			return nil, errors.Wrapf(err, "error configuring runtime")
		}
	}
	if rootlessConfigPath != "" {
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

		if !foundConfig {
			os.MkdirAll(filepath.Dir(rootlessConfigPath), 0755)
			file, err := os.OpenFile(rootlessConfigPath, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0666)
			if err != nil && !os.IsExist(err) {
				return nil, errors.Wrapf(err, "cannot open file %s", rootlessConfigPath)
			}
			if err == nil {
				defer file.Close()
				enc := toml.NewEncoder(file)
				if err := enc.Encode(runtime.config); err != nil {
					os.Remove(rootlessConfigPath)
				}
			}
		}
	}
	if err := makeRuntime(ctx, runtime); err != nil {
		return nil, err
	}
	return runtime, nil
}

// Make a new runtime based on the given configuration
// Sets up containers/storage, state store, OCI runtime
func makeRuntime(ctx context.Context, runtime *Runtime) (err error) {
	// Backward compatibility for `runtime_path`
	if runtime.config.RuntimePath != nil {
		// Don't print twice in rootless mode.
		if os.Geteuid() == 0 {
			logrus.Warningf("The configuration is using `runtime_path`, which is deprecated and will be removed in future.  Please use `runtimes` and `runtime`")
			logrus.Warningf("If you are using both `runtime_path` and `runtime`, the configuration from `runtime_path` is used")
		}

		// Transform `runtime_path` into `runtimes` and `runtime`.
		name := filepath.Base(runtime.config.RuntimePath[0])
		runtime.config.OCIRuntime = name
		runtime.config.OCIRuntimes = map[string][]string{name: runtime.config.RuntimePath}
	}

	// Find a working OCI runtime binary
	foundRuntime := false
	// If runtime is an absolute path, then use it as it is.
	if runtime.config.OCIRuntime != "" && runtime.config.OCIRuntime[0] == '/' {
		foundRuntime = true
		runtime.ociRuntimePath = OCIRuntimePath{Name: filepath.Base(runtime.config.OCIRuntime), Paths: []string{runtime.config.OCIRuntime}}
		stat, err := os.Stat(runtime.config.OCIRuntime)
		if err != nil {
			if os.IsNotExist(err) {
				return errors.Wrapf(err, "the specified OCI runtime %s does not exist", runtime.config.OCIRuntime)
			}
			return errors.Wrapf(err, "cannot stat the OCI runtime path %s", runtime.config.OCIRuntime)
		}
		if !stat.Mode().IsRegular() {
			return fmt.Errorf("the specified OCI runtime %s is not a valid file", runtime.config.OCIRuntime)
		}
	} else {
		// If not, look it up in the configuration.
		paths := runtime.config.OCIRuntimes[runtime.config.OCIRuntime]
		if paths != nil {
			for _, path := range paths {
				stat, err := os.Stat(path)
				if err != nil {
					if os.IsNotExist(err) {
						continue
					}
					return errors.Wrapf(err, "cannot stat %s", path)
				}
				if !stat.Mode().IsRegular() {
					continue
				}
				foundRuntime = true
				runtime.ociRuntimePath = OCIRuntimePath{Name: runtime.config.OCIRuntime, Paths: []string{path}}
				break
			}
		}
	}
	if !foundRuntime {
		return errors.Wrapf(ErrInvalidArg,
			"could not find a working binary (configured options: %v)",
			runtime.config.OCIRuntimes)
	}

	// Find a working conmon binary
	foundConmon := false
	for _, path := range runtime.config.ConmonPath {
		stat, err := os.Stat(path)
		if err != nil {
			continue
		}
		if stat.IsDir() {
			continue
		}
		foundConmon = true
		runtime.conmonPath = path
		break
	}
	if !foundConmon {
		return errors.Wrapf(ErrInvalidArg,
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
		return errors.Wrapf(ErrInvalidArg, "SQLite state is currently disabled")
	case BoltDBStateStore:
		dbPath := filepath.Join(runtime.config.StaticDir, "bolt_state.db")

		state, err := NewBoltState(dbPath, runtime)
		if err != nil {
			return err
		}
		runtime.state = state
	default:
		return errors.Wrapf(ErrInvalidArg, "unrecognized state type passed")
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
	} else {
		store, err = storage.GetStore(runtime.config.StorageConfig)
		if err != nil {
			return err
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
	}

	runtime.store = store
	is.Transport.SetStore(store)

	// Set up image runtime and store in runtime
	ir := image.NewImageRuntimeFromStore(runtime.store)

	runtime.imageRuntime = ir

	// Setting signaturepolicypath
	ir.SignaturePolicyPath = runtime.config.SignaturePolicyPath

	// Set logfile path for events
	ir.EventsLogFilePath = runtime.config.EventsLogFilePath
	// Set logger type
	ir.EventsLogger = runtime.config.EventsLogger

	// Setup the eventer
	eventer, err := runtime.newEventer()
	if err != nil {
		return err
	}
	runtime.eventer = eventer
	ir.Eventer = eventer

	// Set up a storage service for creating container root filesystems from
	// images
	storageService, err := getStorageService(runtime.store)
	if err != nil {
		return err
	}
	runtime.storageService = storageService

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

	supportsJSON := false
	for _, r := range runtime.config.RuntimeSupportsJSON {
		if r == runtime.config.OCIRuntime {
			supportsJSON = true
			break
		}
	}

	// Make an OCI runtime to perform container operations
	ociRuntime, err := newOCIRuntime(runtime.ociRuntimePath,
		runtime.conmonPath, runtime.config.ConmonEnvVars,
		runtime.config.CgroupManager, runtime.config.TmpDir,
		runtime.config.MaxLogSize, runtime.config.NoPivotRoot,
		runtime.config.EnablePortReservation,
		supportsJSON)
	if err != nil {
		return err
	}
	runtime.ociRuntime = ociRuntime

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

	// Set up a firewall backend
	backendType := ""
	if rootless.IsRootless() {
		backendType = "none"
	}
	fwBackend, err := firewall.GetBackend(backendType)
	if err != nil {
		return err
	}
	runtime.firewallBackend = fwBackend

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

	lockPath := DefaultSHMLockPath
	if rootless.IsRootless() {
		lockPath = fmt.Sprintf("%s_%d", DefaultRootlessSHMLockPath, rootless.GetRootlessUID())
	}
	// Set up the lock manager
	manager, err := lock.OpenSHMLockManager(lockPath, runtime.config.NumLocks)
	if err != nil {
		if os.IsNotExist(errors.Cause(err)) {
			manager, err = lock.NewSHMLockManager(lockPath, runtime.config.NumLocks)
			if err != nil {
				return errors.Wrapf(err, "failed to get new shm lock manager")
			}
		} else if errors.Cause(err) == syscall.ERANGE && runtime.doRenumber {
			logrus.Debugf("Number of locks does not match - removing old locks")

			// ERANGE indicates a lock numbering mismatch.
			// Since we're renumbering, this is not fatal.
			// Remove the earlier set of locks and recreate.
			if err := os.Remove(filepath.Join("/dev/shm", lockPath)); err != nil {
				return errors.Wrapf(err, "error removing libpod locks file %s", lockPath)
			}

			manager, err = lock.NewSHMLockManager(lockPath, runtime.config.NumLocks)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}
	runtime.lockManager = manager

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
		return nil, ErrRuntimeStopped
	}

	config := new(RuntimeConfig)

	// Copy so the caller won't be able to modify the actual config
	if err := JSONDeepCopy(r.config, config); err != nil {
		return nil, errors.Wrapf(err, "error copying config")
	}

	return config, nil
}

// Shutdown shuts down the runtime and associated containers and storage
// If force is true, containers and mounted storage will be shut down before
// cleaning up; if force is false, an error will be returned if there are
// still containers running or mounted
func (r *Runtime) Shutdown(force bool) error {
	r.lock.Lock()
	defer r.lock.Unlock()

	if !r.valid {
		return ErrRuntimeStopped
	}

	r.valid = false

	// Shutdown all containers if --force is given
	if force {
		ctrs, err := r.state.AllContainers()
		if err != nil {
			logrus.Errorf("Error retrieving containers from database: %v", err)
		} else {
			for _, ctr := range ctrs {
				if err := ctr.StopWithTimeout(CtrRemoveTimeout); err != nil {
					logrus.Errorf("Error stopping container %s: %v", ctr.ID(), err)
				}
			}
		}
	}

	var lastError error
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
func (r *Runtime) Info() ([]InfoData, error) {
	info := []InfoData{}
	// get host information
	hostInfo, err := r.hostInfo()
	if err != nil {
		return nil, errors.Wrapf(err, "error getting host info")
	}
	info = append(info, InfoData{Type: "host", Data: hostInfo})

	// get store information
	storeInfo, err := r.storeInfo()
	if err != nil {
		return nil, errors.Wrapf(err, "error getting store info")
	}
	info = append(info, InfoData{Type: "store", Data: storeInfo})

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
	info = append(info, InfoData{Type: "registries", Data: registries})
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
			if errors.Cause(err) != ErrNoSuchCtr {
				return "", err
			}
		}
		// Make sure pod with this name does not exist
		if _, err := r.state.LookupPod(name); err == nil {
			continue
		} else {
			if errors.Cause(err) != ErrNoSuchPod {
				return "", err
			}
		}
		return name, nil
	}
	// The code should never reach here.
}

// ImageRuntime returns the imageruntime for image resolution
func (r *Runtime) ImageRuntime() *image.Runtime {
	return r.imageRuntime
}

// SystemContext returns the imagecontext
func (r *Runtime) SystemContext() *types.SystemContext {
	return r.imageContext
}
