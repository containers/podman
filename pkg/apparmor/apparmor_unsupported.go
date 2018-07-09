// +build !linux !apparmor

package apparmor

// InstallDefault generates a default profile in a temp directory determined by
// os.TempDir(), then loads the profile into the kernel using 'apparmor_parser'.
func InstallDefault(name string) error {
	return ErrApparmorUnsupported
}

// IsLoaded checks if a profile with the given name has been loaded into the
// kernel.
func IsLoaded(name string) (bool, error) {
	return false, ErrApparmorUnsupported
}
