//go:build !linux && !windows

package specgen

func shouldResolveWinPaths() bool {
	return false
}

func shouldResolveUnixWinVariant(path string) bool {
	return false
}

func resolveRelativeOnWindows(path string) string {
	return path
}

func winPathExists(path string) bool {
	return false
}
