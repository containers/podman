package config

import (
	"os"

	"github.com/containers/storage/pkg/unshare"
	selinux "github.com/opencontainers/selinux/go-selinux"
)

func selinuxEnabled() bool {
	return selinux.GetEnabled()
}

func customConfigFile() (string, error) {
	if path, found := os.LookupEnv("CONTAINERS_CONF"); found {
		return path, nil
	}
	if unshare.IsRootless() {
		path, err := rootlessConfigPath()
		if err != nil {
			return "", err
		}
		return path, nil
	}
	return OverrideContainersConfig, nil
}

func ifRootlessConfigPath() (string, error) {
	if unshare.IsRootless() {
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
