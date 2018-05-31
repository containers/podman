package sysregistries

import (
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/containers/image/types"
	"io/ioutil"
	"path/filepath"
)

// systemRegistriesConfPath is the path to the system-wide registry configuration file
// and is used to add/subtract potential registries for obtaining images.
// You can override this at build time with
// -ldflags '-X github.com/containers/image/sysregistries.systemRegistriesConfPath=$your_path'
var systemRegistriesConfPath = builtinRegistriesConfPath

// builtinRegistriesConfPath is the path to registry configuration file
// DO NOT change this, instead see systemRegistriesConfPath above.
const builtinRegistriesConfPath = "/etc/containers/registries.conf"

type registries struct {
	Registries []string `toml:"registries"`
}

type tomlConfig struct {
	Registries struct {
		Search   registries `toml:"search"`
		Insecure registries `toml:"insecure"`
		Block    registries `toml:"block"`
	} `toml:"registries"`
}

// normalizeRegistries removes trailing slashes from registries, which is a
// common pitfall when configuring registries (e.g., "docker.io/library/).
func normalizeRegistries(regs *registries) {
	for i := range regs.Registries {
		regs.Registries[i] = strings.TrimRight(regs.Registries[i], "/")
	}
}

// Reads the global registry file from the filesystem. Returns
// a byte array
func readRegistryConf(sys *types.SystemContext) ([]byte, error) {
	return ioutil.ReadFile(RegistriesConfPath(sys))
}

// For mocking in unittests
var readConf = readRegistryConf

// Loads the registry configuration file from the filesystem and
// then unmarshals it.  Returns the unmarshalled object.
func loadRegistryConf(sys *types.SystemContext) (*tomlConfig, error) {
	config := &tomlConfig{}

	configBytes, err := readConf(sys)
	if err != nil {
		return nil, err
	}

	err = toml.Unmarshal(configBytes, &config)
	normalizeRegistries(&config.Registries.Search)
	normalizeRegistries(&config.Registries.Insecure)
	normalizeRegistries(&config.Registries.Block)
	return config, err
}

// GetRegistries returns an array of strings that contain the names
// of the registries as defined in the system-wide
// registries file.  it returns an empty array if none are
// defined
func GetRegistries(sys *types.SystemContext) ([]string, error) {
	config, err := loadRegistryConf(sys)
	if err != nil {
		return nil, err
	}
	return config.Registries.Search.Registries, nil
}

// GetInsecureRegistries returns an array of strings that contain the names
// of the insecure registries as defined in the system-wide
// registries file.  it returns an empty array if none are
// defined
func GetInsecureRegistries(sys *types.SystemContext) ([]string, error) {
	config, err := loadRegistryConf(sys)
	if err != nil {
		return nil, err
	}
	return config.Registries.Insecure.Registries, nil
}

// RegistriesConfPath is the path to the system-wide registry configuration file
func RegistriesConfPath(ctx *types.SystemContext) string {
	path := systemRegistriesConfPath
	if ctx != nil {
		if ctx.SystemRegistriesConfPath != "" {
			path = ctx.SystemRegistriesConfPath
		} else if ctx.RootForImplicitAbsolutePaths != "" {
			path = filepath.Join(ctx.RootForImplicitAbsolutePaths, systemRegistriesConfPath)
		}
	}
	return path
}
