package remoteclientconfig

import (
	"os"
	"path/filepath"

	"github.com/docker/docker/pkg/homedir"
)

func getConfigFilePath() string {
	path := os.Getenv("XDG_CONFIG_HOME")
	if path == "" {
		homeDir := homedir.Get()
		path = filepath.Join(homeDir, ".config")
	}
	return filepath.Join(path, "containers", remoteConfigFileName)
}
