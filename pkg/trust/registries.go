package trust

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/containers/image/v5/types"
	"github.com/docker/docker/pkg/homedir"
	"github.com/ghodss/yaml"
)

// registryConfiguration is one of the files in registriesDirPath configuring lookaside locations, or the result of merging them all.
// NOTE: Keep this in sync with docs/registries.d.md!
type registryConfiguration struct {
	DefaultDocker *registryNamespace `json:"default-docker"`
	// The key is a namespace, using fully-expanded Docker reference format or parent namespaces (per dockerReference.PolicyConfiguration*),
	Docker map[string]registryNamespace `json:"docker"`
}

// registryNamespace defines lookaside locations for a single namespace.
type registryNamespace struct {
	SigStore        string `json:"sigstore"`         // For reading, and if SigStoreStaging is not present, for writing.
	SigStoreStaging string `json:"sigstore-staging"` // For writing only.
}

// systemRegistriesDirPath is the path to registries.d.
const systemRegistriesDirPath = "/etc/containers/registries.d"

// userRegistriesDir is the path to the per user registries.d.
var userRegistriesDir = filepath.FromSlash(".config/containers/registries.d")

// RegistriesDirPath returns a path to registries.d
func RegistriesDirPath(sys *types.SystemContext) string {
	if sys != nil && sys.RegistriesDirPath != "" {
		return sys.RegistriesDirPath
	}
	userRegistriesDirPath := filepath.Join(homedir.Get(), userRegistriesDir)
	if _, err := os.Stat(userRegistriesDirPath); err == nil {
		return userRegistriesDirPath
	}
	if sys != nil && sys.RootForImplicitAbsolutePaths != "" {
		return filepath.Join(sys.RootForImplicitAbsolutePaths, systemRegistriesDirPath)
	}

	return systemRegistriesDirPath
}

// loadAndMergeConfig loads registries.d configuration files in dirPath
func loadAndMergeConfig(dirPath string) (*registryConfiguration, error) {
	mergedConfig := registryConfiguration{Docker: map[string]registryNamespace{}}
	dockerDefaultMergedFrom := ""
	nsMergedFrom := map[string]string{}

	dir, err := os.Open(dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &mergedConfig, nil
		}
		return nil, err
	}
	configNames, err := dir.Readdirnames(0)
	if err != nil {
		return nil, err
	}
	for _, configName := range configNames {
		if !strings.HasSuffix(configName, ".yaml") {
			continue
		}
		configPath := filepath.Join(dirPath, configName)
		configBytes, err := ioutil.ReadFile(configPath)
		if err != nil {
			return nil, err
		}
		var config registryConfiguration
		err = yaml.Unmarshal(configBytes, &config)
		if err != nil {
			return nil, fmt.Errorf("error parsing %s: %w", configPath, err)
		}
		if config.DefaultDocker != nil {
			if mergedConfig.DefaultDocker != nil {
				return nil, fmt.Errorf(`error parsing signature storage configuration: "default-docker" defined both in "%s" and "%s"`,
					dockerDefaultMergedFrom, configPath)
			}
			mergedConfig.DefaultDocker = config.DefaultDocker
			dockerDefaultMergedFrom = configPath
		}
		for nsName, nsConfig := range config.Docker { // includes config.Docker == nil
			if _, ok := mergedConfig.Docker[nsName]; ok {
				return nil, fmt.Errorf(`error parsing signature storage configuration: "docker" namespace "%s" defined both in "%s" and "%s"`,
					nsName, nsMergedFrom[nsName], configPath)
			}
			mergedConfig.Docker[nsName] = nsConfig
			nsMergedFrom[nsName] = configPath
		}
	}
	return &mergedConfig, nil
}

// haveMatchRegistry returns configuration from registryConfigs that is configured for key.
func haveMatchRegistry(key string, registryConfigs *registryConfiguration) *registryNamespace {
	searchKey := key
	if !strings.Contains(searchKey, "/") {
		val, exists := registryConfigs.Docker[searchKey]
		if exists {
			return &val
		}
	}
	for range strings.Split(key, "/") {
		val, exists := registryConfigs.Docker[searchKey]
		if exists {
			return &val
		}
		if strings.Contains(searchKey, "/") {
			searchKey = searchKey[:strings.LastIndex(searchKey, "/")]
		}
	}
	return registryConfigs.DefaultDocker
}
