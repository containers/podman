// +build !remoteclient

package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/containers/libpod/libpod/define"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func outputError(err error) {
	if MainGlobalOpts.LogLevel == "debug" {
		logrus.Errorf(err.Error())
	} else {
		ee, ok := err.(*exec.ExitError)
		if ok {
			if status, ok := ee.Sys().(syscall.WaitStatus); ok {
				exitCode = status.ExitStatus()
			}
		}
		fmt.Fprintln(os.Stderr, "Error:", err.Error())
	}
}

func setExitCode(err error) int {
	cause := errors.Cause(err)
	switch cause {
	case define.ErrNoSuchCtr:
		return 1
	case define.ErrCtrStateInvalid:
		return 2
	}
	return exitCode
}
