package main

import (
	"syscall"

	"github.com/sirupsen/logrus"
)

func checkRLimits() {
	var rLimitNoFile syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimitNoFile); err != nil {
		logrus.Debugf("Error getting RLIMITS_NOFILE: %s", err)
		return
	}

	logrus.Debugf("Got RLIMITS_NOFILE: cur=%d, max=%d", rLimitNoFile.Cur, rLimitNoFile.Max)
}

func earlyInitHook() {
	checkRLimits()
}
