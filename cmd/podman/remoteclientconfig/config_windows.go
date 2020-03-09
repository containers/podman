package remoteclientconfig

import (
	"path/filepath"

	"github.com/containers/storage/pkg/homedir"
)

func getConfigFilePath() string {
	homeDir := homedir.Get()
	return filepath.Join(homeDir, "AppData", "podman", remoteConfigFileName)
}
