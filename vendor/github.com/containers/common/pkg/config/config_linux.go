package config

import (
	"os"

	"github.com/containers/storage/pkg/unshare"
	selinux "github.com/opencontainers/selinux/go-selinux"
)

const (
	// OverrideContainersConfig holds the default config path overridden by the root user
	OverrideContainersConfig = "/etc/" + _configPath

	// DefaultContainersConfig holds the default containers config path
	DefaultContainersConfig = "/usr/share/" + _configPath

	// DefaultSignaturePolicyPath is the default value for the
	// policy.json file.
	DefaultSignaturePolicyPath = "/etc/containers/policy.json"
)

func selinuxEnabled() bool {
	return selinux.GetEnabled()
}

func customConfigFile() (string, error) {
	if path, found := os.LookupEnv("CONTAINERS_CONF"); found {
		return path, nil
	}
	if unshare.GetRootlessUID() > 0 {
		path, err := rootlessConfigPath()
		if err != nil {
			return "", err
		}
		return path, nil
	}
	return OverrideContainersConfig, nil
}

func ifRootlessConfigPath() (string, error) {
	if unshare.GetRootlessUID() > 0 {
		path, err := rootlessConfigPath()
		if err != nil {
			return "", err
		}
		return path, nil
	}
	return "", nil
}

var defaultHelperBinariesDir = []string{
	"/usr/local/libexec/podman",
	"/usr/local/lib/podman",
	"/usr/libexec/podman",
	"/usr/lib/podman",
}
