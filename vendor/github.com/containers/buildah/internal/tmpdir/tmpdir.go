package tmpdir

import (
	"os"
	"path/filepath"

	"github.com/containers/common/pkg/config"
	"github.com/sirupsen/logrus"
)

// GetTempDir returns the path of the preferred temporary directory on the host.
func GetTempDir() string {
	if tmpdir, ok := os.LookupEnv("TMPDIR"); ok {
		abs, err := filepath.Abs(tmpdir)
		if err == nil {
			return abs
		}
		logrus.Warnf("ignoring TMPDIR from environment, evaluating it: %v", err)
	}
	if containerConfig, err := config.Default(); err == nil {
		if tmpdir, err := containerConfig.ImageCopyTmpDir(); err == nil {
			return tmpdir
		}
	}
	return "/var/tmp"
}
