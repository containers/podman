package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

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
		fmt.Fprintln(os.Stderr, "Error:", err.Error())
	}
}
