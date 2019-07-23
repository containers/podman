package define

import (
	"github.com/pkg/errors"
)

const (
	// ExecErrorCodeGeneric is the default error code to return from an exec session if libpod failed
	// prior to calling the runtime
	ExecErrorCodeGeneric = 125
	// ExecErrorCodeCannotInvoke is the error code to return when the runtime fails to invoke a command
	// an example of this can be found by trying to execute a directory:
	// `podman exec -l /etc`
	ExecErrorCodeCannotInvoke = 126
	// ExecErrorCodeNotFound is the error code to return when a command cannot be found
	ExecErrorCodeNotFound = 127
)

// TranslateExecErrorToExitCode takes an error and checks whether it
// has a predefined exit code associated. If so, it returns that, otherwise it returns
// the exit code originally stated in libpod.Exec()
func TranslateExecErrorToExitCode(originalEC int, err error) int {
	if errors.Cause(err) == ErrOCIRuntimePermissionDenied {
		return ExecErrorCodeCannotInvoke
	}
	if errors.Cause(err) == ErrOCIRuntimeNotFound {
		return ExecErrorCodeNotFound
	}
	return originalEC
}
