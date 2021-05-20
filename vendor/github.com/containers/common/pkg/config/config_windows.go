package config

import "os"

// podman remote clients on windows cannot use unshare.isRootless() to determine the configuration file locations.
func customConfigFile() (string, error) {
	if path, found := os.LookupEnv("CONTAINERS_CONF"); found {
		return path, nil
	}
	return os.Getenv("APPDATA") + "\\containers\\containers.conf", nil
}

func ifRootlessConfigPath() (string, error) {
	return os.Getenv("APPDATA") + "\\containers\\containers.conf", nil
}
