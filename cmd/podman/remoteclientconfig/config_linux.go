package remoteclientconfig

import (
	"path/filepath"

	"github.com/docker/docker/pkg/homedir"
)

func getConfigFilePath() string {
	homeDir := homedir.Get()
	return filepath.Join(homeDir, ".config", "containers", remoteConfigFileName)
}
