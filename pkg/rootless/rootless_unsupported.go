// +build !linux

package rootless

import (
	"os"

	"github.com/pkg/errors"
)

// IsRootless returns false on all non-linux platforms
func IsRootless() bool {
	return false
}

// BecomeRootInUserNS is a stub function that always returns false and an
// error on unsupported OS's
func BecomeRootInUserNS() (bool, int, error) {
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

// GetUserNSForPid returns an open FD for the first direct child user namespace that created the process
func GetUserNSForPid(pid uint) (*os.File, error) {
	return nil, errors.New("this function is not supported on this os")
}
