package config

import "os"

func customConfigFile() (string, error) {
	if path, found := os.LookupEnv("CONTAINERS_CONF"); found {
		return path, nil
	}
	return os.Getenv("APPDATA") + "\\containers\\containers.conf", nil
}
