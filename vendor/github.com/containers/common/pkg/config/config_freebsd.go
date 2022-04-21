package config

import (
	"os"
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
