package remoteclientconfig

const remoteConfigFileName string = "podman-remote.conf"

// RemoteConfig describes the podman remote configuration file
type RemoteConfig struct {
	Connections map[string]RemoteConnection
}

// RemoteConnection describes the attributes of a podman-remote endpoint
type RemoteConnection struct {
	Destination  string `toml:"destination"`
	Username     string `toml:"username"`
	IsDefault    bool   `toml:"default"`
	Port         int    `toml:"port"`
	IdentityFile string `toml:"identity_file"`
	IgnoreHosts  bool   `toml:"ignore_hosts"`
}

// GetConfigFilePath is a simple helper to export the configuration file's
// path based on arch, etc
func GetConfigFilePath() string {
	return getConfigFilePath()
}
