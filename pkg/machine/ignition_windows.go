//go:build windows
// +build windows

package machine

func getLocalTimeZone() (string, error) {
	return "", nil
}
