package machine

import "errors"

var (
	ErrNoSuchVM         = errors.New("VM does not exist")
	ErrWrongState       = errors.New("VM in wrong state to perform action")
	ErrVMAlreadyExists  = errors.New("VM already exists")
	ErrVMAlreadyRunning = errors.New("VM already running or starting")
	ErrMultipleActiveVM = errors.New("only one VM can be active at a time")
	ErrNotImplemented   = errors.New("functionality not implemented")
)
