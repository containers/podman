package config

import "os"

const (
	// _configPath is the path to the containers/containers.conf
	// inside a given config directory.
	_configPath = "containers\\containers.conf"

	// DefaultContainersConfig holds the default containers config path
	DefaultContainersConfig = ""

	// DefaultSignaturePolicyPath is the default value for the
	// policy.json file.
	DefaultSignaturePolicyPath = "/etc/containers/policy.json"

	// Mount type for mounting host dir
	_typeBind = "bind"
)

// userConfigPath returns the path to the users local config that is
// not shared with other users. It uses $APPDATA/containers...
func userConfigPath() (string, error) {
	return os.Getenv("APPDATA") + _configPath, nil
}

// overrideContainersConfigPath returns the path to the system wide
// containers config folder. It users $PROGRAMDATA/containers...
func overrideContainersConfigPath() (string, error) {
	return os.Getenv("ProgramData") + _configPath, nil
}

var defaultHelperBinariesDir = []string{
	"C:\\Program Files\\RedHat\\Podman",
}
