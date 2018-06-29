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
func BecomeRootInUserNS() (bool, error) {
	return false, errors.New("this function is not supported on this os")
}
