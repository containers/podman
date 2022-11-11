package config

import (
	"os"
)

const (
	// OverrideContainersConfig holds the default config path overridden by the root user
	OverrideContainersConfig = "/usr/local/etc/" + _configPath

	// DefaultContainersConfig holds the default containers config path
	DefaultContainersConfig = "/usr/local/share/" + _configPath

	// DefaultSignaturePolicyPath is the default value for the
	// policy.json file.
	DefaultSignaturePolicyPath = "/usr/local/etc/containers/policy.json"
)

// podman remote clients on freebsd cannot use unshare.isRootless() to determine the configuration file locations.
func customConfigFile() (string, error) {
	if path, found := os.LookupEnv("CONTAINERS_CONF"); found {
		return path, nil
	}
	return rootlessConfigPath()
}

func ifRootlessConfigPath() (string, error) {
	return rootlessConfigPath()
}

var defaultHelperBinariesDir = []string{
	"/usr/local/bin",
	"/usr/local/libexec/podman",
	"/usr/local/lib/podman",
	"/usr/local/libexec/podman",
	"/usr/local/lib/podman",
}
