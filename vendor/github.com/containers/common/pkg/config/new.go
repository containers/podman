package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/BurntSushi/toml"
	"github.com/containers/storage/pkg/fileutils"
	"github.com/sirupsen/logrus"
)

var (
	cachedConfigError error
	cachedConfigMutex sync.Mutex
	cachedConfig      *Config
)

const (
	containersConfEnv         = "CONTAINERS_CONF"
	containersConfOverrideEnv = containersConfEnv + "_OVERRIDE"
)

// Options to use when loading a Config via New().
type Options struct {
	// Attempt to load the following config modules.
	Modules []string

	// Set the loaded config as the default one which can later on be
	// accessed via Default().
	SetDefault bool

	// Additional configs to load.  An internal only field to make the
	// behavior observable and testable in unit tests.
	additionalConfigs []string
}

// New returns a Config as described in the containers.conf(5) man page.
func New(options *Options) (*Config, error) {
	if options == nil {
		options = &Options{}
	} else if options.SetDefault {
		cachedConfigMutex.Lock()
		defer cachedConfigMutex.Unlock()
	}
	return newLocked(options)
}

// Default returns the default container config.  If no default config has been
// set yet, a new config will be loaded by New() and set as the default one.
// All callers are expected to use the returned Config read only.  Changing
// data may impact other call sites.
func Default() (*Config, error) {
	cachedConfigMutex.Lock()
	defer cachedConfigMutex.Unlock()
	if cachedConfig != nil || cachedConfigError != nil {
		return cachedConfig, cachedConfigError
	}
	cachedConfig, cachedConfigError = newLocked(&Options{SetDefault: true})
	return cachedConfig, cachedConfigError
}

// A helper function for New() expecting the caller to hold the
// cachedConfigMutex if options.SetDefault is set..
func newLocked(options *Options) (*Config, error) {
	// Start with the built-in defaults
	config, err := defaultConfig()
	if err != nil {
		return nil, err
	}

	// Now, gather the system configs and merge them as needed.
	configs, err := systemConfigs()
	if err != nil {
		return nil, fmt.Errorf("finding config on system: %w", err)
	}

	for _, path := range configs {
		// Merge changes in later configs with the previous configs.
		// Each config file that specified fields, will override the
		// previous fields.
		if err = readConfigFromFile(path, config, true); err != nil {
			return nil, fmt.Errorf("reading system config %q: %w", path, err)
		}
		logrus.Debugf("Merged system config %q", path)
		logrus.Tracef("%+v", config)
	}

	modules, err := options.modules()
	if err != nil {
		return nil, err
	}
	config.loadedModules = modules

	options.additionalConfigs = append(options.additionalConfigs, modules...)

	// The _OVERRIDE variable _must_ always win.  That's a contract we need
	// to honor (for the Podman CI).
	if path := os.Getenv(containersConfOverrideEnv); path != "" {
		if err := fileutils.Exists(path); err != nil {
			return nil, fmt.Errorf("%s file: %w", containersConfOverrideEnv, err)
		}
		options.additionalConfigs = append(options.additionalConfigs, path)
	}

	// If the caller specified a config path to use, then we read it to
	// override the system defaults.
	for _, add := range options.additionalConfigs {
		if add == "" {
			continue
		}
		// readConfigFromFile reads in container config in the specified
		// file and then merge changes with the current default.
		if err := readConfigFromFile(add, config, false); err != nil {
			return nil, fmt.Errorf("reading additional config %q: %w", add, err)
		}
		logrus.Debugf("Merged additional config %q", add)
		logrus.Tracef("%+v", config)
	}
	config.addCAPPrefix()

	if err := config.Validate(); err != nil {
		return nil, err
	}

	if err := config.setupEnv(); err != nil {
		return nil, err
	}

	if options.SetDefault {
		cachedConfig = config
		cachedConfigError = nil
	}

	return config, nil
}

// NewConfig creates a new Config. It starts with an empty config and, if
// specified, merges the config at `userConfigPath` path.
//
// Deprecated: use new instead.
func NewConfig(userConfigPath string) (*Config, error) {
	return New(&Options{additionalConfigs: []string{userConfigPath}})
}

// Returns the list of configuration files, if they exist in order of hierarchy.
// The files are read in order and each new file can/will override previous
// file settings.
func systemConfigs() (configs []string, finalErr error) {
	if path := os.Getenv(containersConfEnv); path != "" {
		if err := fileutils.Exists(path); err != nil {
			return nil, fmt.Errorf("%s file: %w", containersConfEnv, err)
		}
		return append(configs, path), nil
	}

	configs = append(configs, DefaultContainersConfig)

	var err error
	path, err := overrideContainersConfigPath()
	if err != nil {
		return nil, err
	}
	configs = append(configs, path)

	configs, err = addConfigs(path+".d", configs)
	if err != nil {
		return nil, err
	}

	path, err = userConfigPath()
	if err != nil {
		return nil, err
	}
	configs = append(configs, path)
	configs, err = addConfigs(path+".d", configs)
	if err != nil {
		return nil, err
	}
	return configs, nil
}

// addConfigs will search one level in the config dirPath for config files
// If the dirPath does not exist, addConfigs will return nil
func addConfigs(dirPath string, configs []string) ([]string, error) {
	newConfigs := []string{}

	err := filepath.WalkDir(dirPath,
		// WalkFunc to read additional configs
		func(path string, d fs.DirEntry, err error) error {
			switch {
			case err != nil:
				// return error (could be a permission problem)
				return err
			case d.IsDir():
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
	if errors.Is(err, os.ErrNotExist) {
		err = nil
	}
	sort.Strings(newConfigs)
	return append(configs, newConfigs...), err
}

// readConfigFromFile reads the specified config file at `path` and attempts to
// unmarshal its content into a Config. The config param specifies the previous
// default config. If the path, only specifies a few fields in the Toml file
// the defaults from the config parameter will be used for all other fields.
func readConfigFromFile(path string, config *Config, ignoreErrNotExist bool) error {
	logrus.Tracef("Reading configuration file %q", path)
	meta, err := toml.DecodeFile(path, config)
	if err != nil {
		if ignoreErrNotExist && errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("decode configuration %v: %w", path, err)
	}
	keys := meta.Undecoded()
	if len(keys) > 0 {
		logrus.Debugf("Failed to decode the keys %q from %q.", keys, path)
	}

	return nil
}
