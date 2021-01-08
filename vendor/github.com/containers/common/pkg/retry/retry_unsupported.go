// +build !linux

package retry

func shouldRestartPlatform(e error) bool {
	return false
}
