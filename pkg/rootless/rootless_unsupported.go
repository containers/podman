// +build !linux

package rootless

import (
	"github.com/pkg/errors"
)

// IsRootless returns false on all non-linux platforms
func IsRootless() bool {
	return false
}

// BecomeRootInUserNS is a stub function that always returns false and an
// error on unsupported OS's
func BecomeRootInUserNS() (bool, int, error) {
	return false, -1, errors.New("this function is not supported on this os1")
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
func JoinNS(pid uint) (bool, int, error) {
	return false, -1, errors.New("this function is not supported on this os2")
}

// JoinNSPath re-exec podman in a new userNS and join the owner user namespace of the
// specified path.
func JoinNSPath(path string) (bool, int, error) {
	return false, -1, errors.New("this function is not supported on this os3")
}

// JoinDirectUserAndMountNS re-exec podman in a new userNS and join the user and mount
// namespace of the specified PID without looking up its parent.  Useful to join directly
// the conmon process.
func JoinDirectUserAndMountNS(pid uint) (bool, int, error) {
	return false, -1, errors.New("this function is not supported on this os4")
}
