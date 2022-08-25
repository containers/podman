//go:build linux || freebsd
// +build linux freebsd

package main

import (
	"fmt"
	"log/syslog"
	"os"

	"github.com/sirupsen/logrus"
	logrusSyslog "github.com/sirupsen/logrus/hooks/syslog"
)

func syslogHook() {
	if !useSyslog {
		return
	}

	hook, err := logrusSyslog.NewSyslogHook("", "", syslog.LOG_INFO, "")
	if err != nil {
		fmt.Fprint(os.Stderr, "Failed to initialize syslog hook: "+err.Error())
		os.Exit(1)
	}
	if err == nil {
		logrus.AddHook(hook)
	}
}
