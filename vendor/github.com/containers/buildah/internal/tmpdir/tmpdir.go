package tmpdir

import (
	"os"
	"path/filepath"

	"github.com/containers/common/pkg/config"
	"github.com/sirupsen/logrus"
)

// GetTempDir returns base for a temporary directory on host.
func GetTempDir() string {
	if tmpdir, ok := os.LookupEnv("TMPDIR"); ok {
		abs, err := filepath.Abs(tmpdir)
		if err == nil {
			return abs
		}
		logrus.Warnf("ignoring TMPDIR from environment, evaluating it: %v", err)
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
