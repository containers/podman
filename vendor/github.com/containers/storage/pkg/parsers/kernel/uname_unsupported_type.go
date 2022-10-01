//go:build !linux && !solaris
// +build !linux,!solaris

package kernel

// Utsname represents the system name structure.
// It is defined here to make it portable as it is available on linux but not
// on windows.
type Utsname struct {
	Release [65]byte
}
