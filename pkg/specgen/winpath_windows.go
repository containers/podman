package specgen

import (
	"github.com/sirupsen/logrus"
	"os"
	"path/filepath"
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
	_, err := os.Stat(path)
	return err == nil
}
