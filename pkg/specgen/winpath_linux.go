package specgen

import (
	"os"

	"github.com/containers/common/pkg/machine"
)

func shouldResolveWinPaths() bool {
	return machine.MachineHostType() == "wsl"
}

func shouldResolveUnixWinVariant(path string) bool {
	_, err := os.Stat(path)
	return err != nil
}

func resolveRelativeOnWindows(path string) string {
	return path
}

func winPathExists(path string) bool {
	return false
}
