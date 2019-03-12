// +build !linux

package rootless

import (
	"github.com/pkg/errors"
)

// IsRootless returns false on all non-linux platforms
func IsRootless() bool {
	return false
}

// BecomeRootInUserNS re-exec podman in a new userNS.  It returns whether podman was re-executed
// into a new user namespace and the return code from the re-executed podman process.
// If podman was re-executed the caller needs to propagate the error code returned by the child
// process.  It is a convenience function for BecomeRootInUserNSWithOpts with a default configuration.
func BecomeRootInUserNS() (bool, int, error) {
	return false, -1, errors.New("this function is not supported on this os")
}

// BecomeRootInUserNS is a stub function that always returns false and an
// error on unsupported OS's
func BecomeRootInUserNSWithOpts(opts *Opts) (bool, int, error) {
	return false, -1, errors.New("this function is not supported on this os")
}

// GetRootlessUID returns the UID of the user in the parent userNS
func GetRootlessUID() int {
	return -1
}

// SetSkipStorageSetup tells the runtime to not setup containers/storage
func SetSkipStorageSetup(bool) {
}

// SkipStorageSetup tells if we should skip the containers/storage setup
func SkipStorageSetup() bool {
	return false
}

// JoinNS re-exec podman in a new userNS and join the user namespace of the specified
// PID.
func JoinNS(pid uint, preserveFDs int) (bool, int, error) {
	return false, -1, errors.New("this function is not supported on this os")
}

// JoinNSPath re-exec podman in a new userNS and join the owner user namespace of the
// specified path.
func JoinNSPath(path string) (bool, int, error) {
	return false, -1, errors.New("this function is not supported on this os")
}

// JoinDirectUserAndMountNSWithOpts re-exec podman in a new userNS and join the user and
// mount namespace of the specified PID without looking up its parent.  Useful to join
// directly the conmon process.
func JoinDirectUserAndMountNSWithOpts(pid uint, opts *Opts) (bool, int, error) {
	return false, -1, errors.New("this function is not supported on this os")
}

// JoinDirectUserAndMountNS re-exec podman in a new userNS and join the user and mount
// namespace of the specified PID without looking up its parent.  Useful to join directly
// the conmon process.  It is a convenience function for JoinDirectUserAndMountNSWithOpts
// with a default configuration.
func JoinDirectUserAndMountNS(pid uint) (bool, int, error) {
	return false, -1, errors.New("this function is not supported on this os")
}

// Argument returns the argument that was set for the rootless session.
func Argument() string {
	return ""
}
