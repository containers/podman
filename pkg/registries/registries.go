package registries

import (
	"os"

	"github.com/containers/image/pkg/sysregistries"
	"github.com/containers/image/types"
	"github.com/pkg/errors"
)

// GetRegistries obtains the list of registries defined in the global registries file.
func GetRegistries() ([]string, error) {
	registryConfigPath := ""
	envOverride := os.Getenv("REGISTRIES_CONFIG_PATH")
	if len(envOverride) > 0 {
		registryConfigPath = envOverride
	}
	searchRegistries, err := sysregistries.GetRegistries(&types.SystemContext{SystemRegistriesConfPath: registryConfigPath})
	if err != nil {
		return nil, errors.Wrapf(err, "unable to parse the registries.conf file")
	}
	return searchRegistries, nil
}

// GetInsecureRegistries obtains the list of insecure registries from the global registration file.
func GetInsecureRegistries() ([]string, error) {
	registryConfigPath := ""
	envOverride := os.Getenv("REGISTRIES_CONFIG_PATH")
	if len(envOverride) > 0 {
		registryConfigPath = envOverride
	}
	registries, err := sysregistries.GetInsecureRegistries(&types.SystemContext{SystemRegistriesConfPath: registryConfigPath})
	if err != nil {
		return nil, errors.Wrapf(err, "unable to parse the registries.conf file")
	}
	return registries, nil
}
