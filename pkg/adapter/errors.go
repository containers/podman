// +build remoteclient

package adapter

import (
	iopodman "github.com/containers/libpod/cmd/podman/varlink"
	"github.com/containers/libpod/libpod/define"
	"github.com/pkg/errors"
)

// TranslateMapErrors translates the errors a typical podman output struct
// from varlink errors to libpod errors
func TranslateMapErrors(failures map[string]error) map[string]error {
	for k, v := range failures {
		failures[k] = TranslateError(v)
	}
	return failures
}

// TranslateError converts a single varlink error to a libpod error
func TranslateError(err error) error {
	switch err.(type) {
	case *iopodman.ContainerNotFound:
		return errors.Wrap(define.ErrNoSuchCtr, err.Error())
	case *iopodman.ErrCtrStopped:
		return errors.Wrap(define.ErrCtrStopped, err.Error())
	case *iopodman.InvalidState:
		return errors.Wrap(define.ErrCtrStateInvalid, err.Error())
	}
	return err
}
