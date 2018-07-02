// +build !linux

package buildah

import (
	"github.com/pkg/errors"
)

func setChildProcess() error {
	return errors.New("function not supported on non-linux systems")
}
