package types

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
	cfg "github.com/containers/storage/pkg/config"
	"github.com/containers/storage/pkg/idtools"
	"github.com/sirupsen/logrus"
)

// TOML-friendly explicit tables used for conversions.
type TomlConfig struct {
	Storage struct {
		Driver              string            `toml:"driver,omitempty"`
		RunRoot             string            `toml:"runroot,omitempty"`
		GraphRoot           string            `toml:"graphroot,omitempty"`
		RootlessStoragePath string            `toml:"rootless_storage_path,omitempty"`
		Options             cfg.OptionsConfig `toml:"options,omitempty"`
	} `toml:"storage"`
}

const (
	// these are default path for run and graph root for rootful users
	// for rootless path is constructed via getRootlessStorageOpts
	defaultRunRoot   string = "/run/containers/storage"
	defaultGraphRoot string = "/var/lib/containers/storage"
)

// defaultConfigFile path to the system wide storage.conf file
var (
	defaultConfigFile         = "/usr/share/containers/storage.conf"
	defaultOverrideConfigFile = "/etc/containers/storage.conf"
	defaultConfigFileSet      = false
	// DefaultStoreOptions is a reasonable default set of options.
	defaultStoreOptions StoreOptions
)

const (
	overlayDriver = "overlay"
	overlay2      = "overlay2"
)

func init() {
	defaultStoreOptions.RunRoot = defaultRunRoot
	defaultStoreOptions.GraphRoot = defaultGraphRoot
	defaultStoreOptions.GraphDriverName = ""

	if _, err := os.Stat(defaultOverrideConfigFile); err == nil {
		// The DefaultConfigFile(rootless) function returns the path
		// of the used storage.conf file, by returning defaultConfigFile
		// If override exists containers/storage uses it by default.
		defaultConfigFile = defaultOverrideConfigFile
		ReloadConfigurationFileIfNeeded(defaultOverrideConfigFile, &defaultStoreOptions)
	} else {
		if !os.IsNotExist(err) {
			logrus.Warningf("Attempting to use %s, %v", defaultConfigFile, err)
		}
		ReloadConfigurationFileIfNeeded(defaultConfigFile, &defaultStoreOptions)
	}
	// reload could set values to empty for run and graph root if config does not contains anything
	if defaultStoreOptions.RunRoot == "" {
		defaultStoreOptions.RunRoot = defaultRunRoot
	}
	if defaultStoreOptions.GraphRoot == "" {
		defaultStoreOptions.GraphRoot = defaultGraphRoot
	}
}

// defaultStoreOptionsIsolated is an internal implementation detail of DefaultStoreOptions to allow testing.
// Everyone but the tests this is intended for should only call DefaultStoreOptions, never this function.
func defaultStoreOptionsIsolated(rootless bool, rootlessUID int, storageConf string) (StoreOptions, error) {
	var (
		defaultRootlessRunRoot   string
		defaultRootlessGraphRoot string
		err                      error
	)
	storageOpts := defaultStoreOptions
	if rootless && rootlessUID != 0 {
		storageOpts, err = getRootlessStorageOpts(rootlessUID, storageOpts)
		if err != nil {
			return storageOpts, err
		}
	}
	_, err = os.Stat(storageConf)
	if err != nil && !os.IsNotExist(err) {
		return storageOpts, err
	}
	if err == nil && !defaultConfigFileSet {
		defaultRootlessRunRoot = storageOpts.RunRoot
		defaultRootlessGraphRoot = storageOpts.GraphRoot
		storageOpts = StoreOptions{}
		reloadConfigurationFileIfNeeded(storageConf, &storageOpts)
		if rootless && rootlessUID != 0 {
			// If the file did not specify a graphroot or runroot,
			// set sane defaults so we don't try and use root-owned
			// directories
			if storageOpts.RunRoot == "" {
				storageOpts.RunRoot = defaultRootlessRunRoot
			}
			if storageOpts.GraphRoot == "" {
				if storageOpts.RootlessStoragePath != "" {
					storageOpts.GraphRoot = storageOpts.RootlessStoragePath
				} else {
					storageOpts.GraphRoot = defaultRootlessGraphRoot
				}
			}
		}
	}
	if storageOpts.RunRoot != "" {
		runRoot, err := expandEnvPath(storageOpts.RunRoot, rootlessUID)
		if err != nil {
			return storageOpts, err
		}
		storageOpts.RunRoot = runRoot
	}
	if storageOpts.GraphRoot != "" {
		graphRoot, err := expandEnvPath(storageOpts.GraphRoot, rootlessUID)
		if err != nil {
			return storageOpts, err
		}
		storageOpts.GraphRoot = graphRoot
	}
	if storageOpts.RootlessStoragePath != "" {
		storagePath, err := expandEnvPath(storageOpts.RootlessStoragePath, rootlessUID)
		if err != nil {
			return storageOpts, err
		}
		storageOpts.RootlessStoragePath = storagePath
	}

	return storageOpts, nil
}

// DefaultStoreOptions returns the default storage ops for containers
func DefaultStoreOptions(rootless bool, rootlessUID int) (StoreOptions, error) {
	storageConf, err := DefaultConfigFile(rootless && rootlessUID != 0)
	if err != nil {
		return defaultStoreOptions, err
	}
	return defaultStoreOptionsIsolated(rootless, rootlessUID, storageConf)
}

// StoreOptions is used for passing initialization options to GetStore(), for
// initializing a Store object and the underlying storage that it controls.
type StoreOptions struct {
	// RunRoot is the filesystem path under which we can store run-time
	// information, such as the locations of active mount points, that we
	// want to lose if the host is rebooted.
	RunRoot string `json:"runroot,omitempty"`
	// GraphRoot is the filesystem path under which we will store the
	// contents of layers, images, and containers.
	GraphRoot string `json:"root,omitempty"`
	// RootlessStoragePath is the storage path for rootless users
	// default $HOME/.local/share/containers/storage
	RootlessStoragePath string `toml:"rootless_storage_path"`
	// GraphDriverName is the underlying storage driver that we'll be
	// using.  It only needs to be specified the first time a Store is
	// initialized for a given RunRoot and GraphRoot.
	GraphDriverName string `json:"driver,omitempty"`
	// GraphDriverOptions are driver-specific options.
	GraphDriverOptions []string `json:"driver-options,omitempty"`
	// UIDMap and GIDMap are used for setting up a container's root filesystem
	// for use inside of a user namespace where UID mapping is being used.
	UIDMap []idtools.IDMap `json:"uidmap,omitempty"`
	GIDMap []idtools.IDMap `json:"gidmap,omitempty"`
	// RootAutoNsUser is the user used to pick a subrange when automatically setting
	// a user namespace for the root user.
	RootAutoNsUser string `json:"root_auto_ns_user,omitempty"`
	// AutoNsMinSize is the minimum size for an automatic user namespace.
	AutoNsMinSize uint32 `json:"auto_userns_min_size,omitempty"`
	// AutoNsMaxSize is the maximum size for an automatic user namespace.
	AutoNsMaxSize uint32 `json:"auto_userns_max_size,omitempty"`
	// PullOptions specifies options to be handed to pull managers
	// This API is experimental and can be changed without bumping the major version number.
	PullOptions map[string]string `toml:"pull_options"`
	// DisableVolatile doesn't allow volatile mounts when it is set.
	DisableVolatile bool `json:"disable-volatile,omitempty"`
}

// isRootlessDriver returns true if the given storage driver is valid for containers running as non root
func isRootlessDriver(driver string) bool {
	validDrivers := map[string]bool{
		"btrfs":    true,
		"overlay":  true,
		"overlay2": true,
		"vfs":      true,
	}
	return validDrivers[driver]
}

// getRootlessStorageOpts returns the storage opts for containers running as non root
func getRootlessStorageOpts(rootlessUID int, systemOpts StoreOptions) (StoreOptions, error) {
	var opts StoreOptions

	dataDir, rootlessRuntime, err := getRootlessDirInfo(rootlessUID)
	if err != nil {
		return opts, err
	}
	opts.RunRoot = rootlessRuntime
	if systemOpts.RootlessStoragePath != "" {
		opts.GraphRoot, err = expandEnvPath(systemOpts.RootlessStoragePath, rootlessUID)
		if err != nil {
			return opts, err
		}
	} else {
		opts.GraphRoot = filepath.Join(dataDir, "containers", "storage")
	}

	if driver := systemOpts.GraphDriverName; isRootlessDriver(driver) {
		opts.GraphDriverName = driver
	}
	if driver := os.Getenv("STORAGE_DRIVER"); driver != "" {
		opts.GraphDriverName = driver
	}
	if opts.GraphDriverName == overlay2 {
		logrus.Warnf("Switching default driver from overlay2 to the equivalent overlay driver.")
		opts.GraphDriverName = overlayDriver
	}

	if opts.GraphDriverName == overlayDriver {
		for _, o := range systemOpts.GraphDriverOptions {
			if strings.Contains(o, "ignore_chown_errors") {
				opts.GraphDriverOptions = append(opts.GraphDriverOptions, o)
				break
			}
		}
	}
	if opts.GraphDriverName == "" {
		opts.GraphDriverName = "vfs"
	}

	if os.Getenv("STORAGE_OPTS") != "" {
		opts.GraphDriverOptions = append(opts.GraphDriverOptions, strings.Split(os.Getenv("STORAGE_OPTS"), ",")...)
	}

	return opts, nil
}

// DefaultStoreOptionsAutoDetectUID returns the default storage ops for containers
func DefaultStoreOptionsAutoDetectUID() (StoreOptions, error) {
	uid := getRootlessUID()
	return DefaultStoreOptions(uid != 0, uid)
}

var prevReloadConfig = struct {
	storeOptions *StoreOptions
	mod          time.Time
	mutex        sync.Mutex
	configFile   string
}{}

// SetDefaultConfigFilePath sets the default configuration to the specified path
func SetDefaultConfigFilePath(path string) {
	defaultConfigFile = path
	defaultConfigFileSet = true
	ReloadConfigurationFileIfNeeded(defaultConfigFile, &defaultStoreOptions)
}

func ReloadConfigurationFileIfNeeded(configFile string, storeOptions *StoreOptions) {
	prevReloadConfig.mutex.Lock()
	defer prevReloadConfig.mutex.Unlock()

	fi, err := os.Stat(configFile)
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Printf("Failed to read %s %v\n", configFile, err.Error())
		}
		return
	}

	mtime := fi.ModTime()
	if prevReloadConfig.storeOptions != nil && prevReloadConfig.mod == mtime && prevReloadConfig.configFile == configFile {
		*storeOptions = *prevReloadConfig.storeOptions
		return
	}

	ReloadConfigurationFile(configFile, storeOptions)

	prevReloadConfig.storeOptions = storeOptions
	prevReloadConfig.mod = mtime
	prevReloadConfig.configFile = configFile
}

// ReloadConfigurationFile parses the specified configuration file and overrides
// the configuration in storeOptions.
func ReloadConfigurationFile(configFile string, storeOptions *StoreOptions) {
	config := new(TomlConfig)

	meta, err := toml.DecodeFile(configFile, &config)
	if err == nil {
		keys := meta.Undecoded()
		if len(keys) > 0 {
			logrus.Warningf("Failed to decode the keys %q from %q.", keys, configFile)
		}
	} else {
		if !os.IsNotExist(err) {
			fmt.Printf("Failed to read %s %v\n", configFile, err.Error())
			return
		}
	}

	// Clear storeOptions of previous settings
	*storeOptions = StoreOptions{}
	if config.Storage.Driver != "" {
		storeOptions.GraphDriverName = config.Storage.Driver
	}
	if os.Getenv("STORAGE_DRIVER") != "" {
		config.Storage.Driver = os.Getenv("STORAGE_DRIVER")
		storeOptions.GraphDriverName = config.Storage.Driver
	}
	if storeOptions.GraphDriverName == overlay2 {
		logrus.Warnf("Switching default driver from overlay2 to the equivalent overlay driver.")
		storeOptions.GraphDriverName = overlayDriver
	}
	if storeOptions.GraphDriverName == "" {
		logrus.Errorf("The storage 'driver' option must be set in %s, guarantee proper operation.", configFile)
	}
	if config.Storage.RunRoot != "" {
		storeOptions.RunRoot = config.Storage.RunRoot
	}
	if config.Storage.GraphRoot != "" {
		storeOptions.GraphRoot = config.Storage.GraphRoot
	}
	if config.Storage.RootlessStoragePath != "" {
		storeOptions.RootlessStoragePath = config.Storage.RootlessStoragePath
	}
	for _, s := range config.Storage.Options.AdditionalImageStores {
		storeOptions.GraphDriverOptions = append(storeOptions.GraphDriverOptions, fmt.Sprintf("%s.imagestore=%s", config.Storage.Driver, s))
	}
	for _, s := range config.Storage.Options.AdditionalLayerStores {
		storeOptions.GraphDriverOptions = append(storeOptions.GraphDriverOptions, fmt.Sprintf("%s.additionallayerstore=%s", config.Storage.Driver, s))
	}
	if config.Storage.Options.Size != "" {
		storeOptions.GraphDriverOptions = append(storeOptions.GraphDriverOptions, fmt.Sprintf("%s.size=%s", config.Storage.Driver, config.Storage.Options.Size))
	}
	if config.Storage.Options.MountProgram != "" {
		storeOptions.GraphDriverOptions = append(storeOptions.GraphDriverOptions, fmt.Sprintf("%s.mount_program=%s", config.Storage.Driver, config.Storage.Options.MountProgram))
	}
	if config.Storage.Options.SkipMountHome != "" {
		storeOptions.GraphDriverOptions = append(storeOptions.GraphDriverOptions, fmt.Sprintf("%s.skip_mount_home=%s", config.Storage.Driver, config.Storage.Options.SkipMountHome))
	}
	if config.Storage.Options.IgnoreChownErrors != "" {
		storeOptions.GraphDriverOptions = append(storeOptions.GraphDriverOptions, fmt.Sprintf("%s.ignore_chown_errors=%s", config.Storage.Driver, config.Storage.Options.IgnoreChownErrors))
	}
	if config.Storage.Options.ForceMask != 0 {
		storeOptions.GraphDriverOptions = append(storeOptions.GraphDriverOptions, fmt.Sprintf("%s.force_mask=%o", config.Storage.Driver, config.Storage.Options.ForceMask))
	}
	if config.Storage.Options.MountOpt != "" {
		storeOptions.GraphDriverOptions = append(storeOptions.GraphDriverOptions, fmt.Sprintf("%s.mountopt=%s", config.Storage.Driver, config.Storage.Options.MountOpt))
	}
	if config.Storage.Options.RemapUser != "" && config.Storage.Options.RemapGroup == "" {
		config.Storage.Options.RemapGroup = config.Storage.Options.RemapUser
	}
	if config.Storage.Options.RemapGroup != "" && config.Storage.Options.RemapUser == "" {
		config.Storage.Options.RemapUser = config.Storage.Options.RemapGroup
	}
	if config.Storage.Options.RemapUser != "" && config.Storage.Options.RemapGroup != "" {
		mappings, err := idtools.NewIDMappings(config.Storage.Options.RemapUser, config.Storage.Options.RemapGroup)
		if err != nil {
			fmt.Printf("Error initializing ID mappings for %s:%s %v\n", config.Storage.Options.RemapUser, config.Storage.Options.RemapGroup, err)
			return
		}
		storeOptions.UIDMap = mappings.UIDs()
		storeOptions.GIDMap = mappings.GIDs()
	}

	uidmap, err := idtools.ParseIDMap([]string{config.Storage.Options.RemapUIDs}, "remap-uids")
	if err != nil {
		fmt.Print(err)
	} else {
		storeOptions.UIDMap = uidmap
	}
	gidmap, err := idtools.ParseIDMap([]string{config.Storage.Options.RemapGIDs}, "remap-gids")
	if err != nil {
		fmt.Print(err)
	} else {
		storeOptions.GIDMap = gidmap
	}
	storeOptions.RootAutoNsUser = config.Storage.Options.RootAutoUsernsUser
	if config.Storage.Options.AutoUsernsMinSize > 0 {
		storeOptions.AutoNsMinSize = config.Storage.Options.AutoUsernsMinSize
	}
	if config.Storage.Options.AutoUsernsMaxSize > 0 {
		storeOptions.AutoNsMaxSize = config.Storage.Options.AutoUsernsMaxSize
	}
	if config.Storage.Options.PullOptions != nil {
		storeOptions.PullOptions = config.Storage.Options.PullOptions
	}

	storeOptions.DisableVolatile = config.Storage.Options.DisableVolatile

	storeOptions.GraphDriverOptions = append(storeOptions.GraphDriverOptions, cfg.GetGraphDriverOptions(storeOptions.GraphDriverName, config.Storage.Options)...)

	if opts, ok := os.LookupEnv("STORAGE_OPTS"); ok {
		storeOptions.GraphDriverOptions = strings.Split(opts, ",")
	}
	if len(storeOptions.GraphDriverOptions) == 1 && storeOptions.GraphDriverOptions[0] == "" {
		storeOptions.GraphDriverOptions = nil
	}
}

func Options() StoreOptions {
	return defaultStoreOptions
}

// Save overwrites the tomlConfig in storage.conf with the given conf
func Save(conf TomlConfig, rootless bool) error {
	configFile, err := DefaultConfigFile(rootless)
	if err != nil {
		return err
	}

	if err = os.Remove(configFile); !os.IsNotExist(err) && err != nil {
		return err
	}

	f, err := os.Create(configFile)
	if err != nil {
		return err
	}

	return toml.NewEncoder(f).Encode(conf)
}

// StorageConfig is used to retrieve the storage.conf toml in order to overwrite it
func StorageConfig(rootless bool) (*TomlConfig, error) {
	config := new(TomlConfig)

	configFile, err := DefaultConfigFile(rootless)
	if err != nil {
		return nil, err
	}

	_, err = toml.DecodeFile(configFile, &config)
	if err != nil {
		return nil, err
	}

	return config, nil
}
