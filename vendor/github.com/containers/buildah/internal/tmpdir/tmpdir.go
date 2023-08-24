package tmpdir

import (
	"os"

	"github.com/containers/common/pkg/config"
)

// GetTempDir returns base for a temporary directory on host.
func GetTempDir() string {
	if tmpdir, ok := os.LookupEnv("TMPDIR"); ok {
		return tmpdir
	}
	containerConfig, err := config.Default()
	if err != nil {
		tmpdir, err := containerConfig.ImageCopyTmpDir()
		if err != nil {
			return tmpdir
		}
	}
	return "/var/tmp"
}
