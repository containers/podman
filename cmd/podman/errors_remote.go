// +build remoteclient

package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/containers/libpod/cmd/podman/varlink"
	"github.com/containers/libpod/libpod/define"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func outputError(err error) {
	if MainGlobalOpts.LogLevel == "debug" {
		logrus.Errorf(err.Error())
	} else {
		if ee, ok := err.(*exec.ExitError); ok {
			if status, ok := ee.Sys().(syscall.WaitStatus); ok {
				exitCode = status.ExitStatus()
			}
		}
		var ne error
		switch e := err.(type) {
		// For some reason golang wont let me list them with commas so listing them all.
		case *iopodman.ImageNotFound:
			ne = errors.New(e.Reason)
		case *iopodman.ContainerNotFound:
			ne = errors.New(e.Reason)
		case *iopodman.PodNotFound:
			ne = errors.New(e.Reason)
		case *iopodman.VolumeNotFound:
			ne = errors.New(e.Reason)
		case *iopodman.InvalidState:
			ne = errors.New(e.Reason)
		case *iopodman.ErrorOccurred:
			ne = errors.New(e.Reason)
		default:
			ne = err
		}
		fmt.Fprintln(os.Stderr, "Error:", ne.Error())
	}
}

func setExitCode(err error) int {
	cause := errors.Cause(err)
	switch e := cause.(type) {
	// For some reason golang wont let me list them with commas so listing them all.
	case *iopodman.ContainerNotFound:
		return 1
	case *iopodman.InvalidState:
		return 2
	default:
		switch e {
		case define.ErrNoSuchCtr:
			return 1
		case define.ErrCtrStateInvalid:
			return 2
		}
	}
	return exitCode
}
