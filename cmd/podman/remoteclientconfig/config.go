package remoteclientconfig

const remoteConfigFileName string = "podman-remote.conf"

// RemoteConfig describes the podman remote configuration file
type RemoteConfig struct {
	Connections map[string]RemoteConnection
}

// RemoteConnection describes the attributes of a podman-remote endpoint
type RemoteConnection struct {
	Destination string `toml:"destination"`
	Username    string `toml:"username"`
	IsDefault   bool   `toml:"default"`
}

// GetConfigFilePath is a simple helper to export the configuration file's
// path based on arch, etc
func GetConfigFilePath() string {
	return getConfigFilePath()
}
