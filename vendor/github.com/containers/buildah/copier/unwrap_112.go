// +build !go113

package copier

import (
	"github.com/pkg/errors"
)

func unwrapError(err error) error {
	return errors.Cause(err)
}
