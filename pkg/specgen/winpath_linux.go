package specgen

import (
	"github.com/containers/common/pkg/machine"
	"github.com/containers/storage/pkg/fileutils"
)

func shouldResolveWinPaths() bool {
	return machine.HostType() == "wsl"
}

func shouldResolveUnixWinVariant(path string) bool {
	return fileutils.Exists(path) != nil
}

func resolveRelativeOnWindows(path string) string {
	return path
}

func winPathExists(path string) bool {
	return false
}
