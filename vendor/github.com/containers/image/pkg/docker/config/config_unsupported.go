// +build !linux
// +build !386 !amd64

package config

func getAuthFromKernelKeyring(registry string) (string, string, error) {
	return "", "", ErrNotSupported
}

func deleteAuthFromKernelKeyring(registry string) error {
	return ErrNotSupported
}

func setAuthToKernelKeyring(registry, username, password string) error {
	return ErrNotSupported
}

func removeAllAuthFromKernelKeyring() error {
	return ErrNotSupported
}
