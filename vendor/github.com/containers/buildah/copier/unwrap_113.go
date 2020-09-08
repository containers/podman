// +build go113

package copier

import (
	stderror "errors"

	"github.com/pkg/errors"
)

func unwrapError(err error) error {
	e := errors.Cause(err)
	for e != nil {
		err = e
		e = errors.Unwrap(err)
	}
	return err
}
