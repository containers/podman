package specgen

import (
	"path/filepath"

	"github.com/containers/storage/pkg/fileutils"
	"github.com/sirupsen/logrus"
)

func shouldResolveUnixWinVariant(path string) bool {
	return true
}

func shouldResolveWinPaths() bool {
	return true
}

func resolveRelativeOnWindows(path string) string {
	ret, err := filepath.Abs(path)
	if err != nil {
		logrus.Debugf("problem resolving possible relative path %q: %s", path, err.Error())
		return path
	}

	return ret
}

func winPathExists(path string) bool {
	return fileutils.Exists(path) == nil
}
