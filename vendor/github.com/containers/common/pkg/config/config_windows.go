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

	// Mount type for mounting host dir
	_typeBind = "bind"
)

// userConfigPath returns the path to the users local config that is
// not shared with other users. It uses $APPDATA/containers...
func userConfigPath() (string, error) {
	return os.Getenv("APPDATA") + "\\containers\\containers.conf", nil
}

var defaultHelperBinariesDir = []string{
	"C:\\Program Files\\RedHat\\Podman",
}
