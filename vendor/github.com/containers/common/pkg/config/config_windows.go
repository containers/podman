package config

import "os"

const (
	// OverrideContainersConfig holds the default config path overridden by the root user
	OverrideContainersConfig = "/etc/" + _configPath

	// DefaultContainersConfig holds the default containers config path
	DefaultContainersConfig = "/usr/share/" + _configPath

	// DefaultSignaturePolicyPath is the default value for the
	// policy.json file.
	DefaultSignaturePolicyPath = "/etc/containers/policy.json"
)

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

var defaultHelperBinariesDir = []string{
	"C:\\Program Files\\RedHat\\Podman",
}
