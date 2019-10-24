package registries

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/containers/image/v5/pkg/sysregistriesv2"
	"github.com/containers/image/v5/types"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/docker/distribution/reference"
	"github.com/pkg/errors"
)

// userRegistriesFile is the path to the per user registry configuration file.
var userRegistriesFile = filepath.Join(os.Getenv("HOME"), ".config/containers/registries.conf")

// SystemRegistriesConfPath returns an appropriate value for types.SystemContext.SystemRegistriesConfPath
// (possibly "", which is not an error), taking into account rootless mode and environment variable overrides.
//
// FIXME: This should be centralized in a global SystemContext initializer inherited throughout the code,
// not haphazardly called throughout the way it is being called now.
func SystemRegistriesConfPath() string {
	if envOverride := os.Getenv("REGISTRIES_CONFIG_PATH"); len(envOverride) > 0 {
		return envOverride
	}

	if rootless.IsRootless() {
		if _, err := os.Stat(userRegistriesFile); err == nil {
			return userRegistriesFile
		}
	}

	return ""
}

func getRegistries() ([]sysregistriesv2.Registry, error) {
	registries, err := sysregistriesv2.GetRegistries(&types.SystemContext{SystemRegistriesConfPath: SystemRegistriesConfPath()})
	if err != nil {
		return nil, errors.Wrapf(err, "unable to parse the registries.conf file")
	}
	return registries, nil
}

// GetRegistries obtains the list of search registries defined in the global registries file.
func GetRegistries() ([]string, error) {
	return sysregistriesv2.UnqualifiedSearchRegistries(&types.SystemContext{SystemRegistriesConfPath: SystemRegistriesConfPath()})
}

// GetBlockedRegistries obtains the list of blocked registries defined in the global registries file.
func GetBlockedRegistries() ([]string, error) {
	var blockedRegistries []string
	registries, err := getRegistries()
	if err != nil {
		return nil, err
	}
	for _, reg := range registries {
		if reg.Blocked {
			blockedRegistries = append(blockedRegistries, reg.Prefix)
		}
	}
	return blockedRegistries, nil
}

// GetInsecureRegistries obtains the list of insecure registries from the global registration file.
func GetInsecureRegistries() ([]string, error) {
	var insecureRegistries []string
	registries, err := getRegistries()
	if err != nil {
		return nil, err
	}
	for _, reg := range registries {
		if reg.Insecure {
			insecureRegistries = append(insecureRegistries, reg.Prefix)
		}
	}
	return insecureRegistries, nil
}

// GetRegistry returns the registry name from a string if specified
func GetRegistry(image string) (string, error) {
	// It is possible to only have the registry name in the format "myregistry/"
	// if so, just trim the "/" from the end and return the registry name
	if strings.HasSuffix(image, "/") {
		return strings.TrimSuffix(image, "/"), nil
	}
	imgRef, err := reference.Parse(image)
	if err != nil {
		return "", err
	}
	return reference.Domain(imgRef.(reference.Named)), nil
}
