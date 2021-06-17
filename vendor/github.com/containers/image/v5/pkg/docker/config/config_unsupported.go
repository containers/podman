// +build !linux
// +build !386 !amd64

package config

func getAuthFromKernelKeyring(registry string) (string, string, error) { //nolint:deadcode,unused
	return "", "", ErrNotSupported
}

func deleteAuthFromKernelKeyring(registry string) error { //nolint:deadcode,unused
	return ErrNotSupported
}

func setAuthToKernelKeyring(registry, username, password string) error { //nolint:deadcode,unused
	return ErrNotSupported
}

func removeAllAuthFromKernelKeyring() error { //nolint:deadcode,unused
	return ErrNotSupported
}
