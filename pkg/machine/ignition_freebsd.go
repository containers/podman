//go:build freebsd

package machine

func getLocalTimeZone() (string, error) {
	return "", nil
}
