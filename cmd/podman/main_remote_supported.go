// +build remoteclient
// +build linux darwin

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/containers/libpod/pkg/util"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func setSyslog() error {
	var err error
	cfgHomeDir := os.Getenv("XDG_CONFIG_HOME")
	if cfgHomeDir == "" {
		if cfgHomeDir, err = util.GetRootlessConfigHomeDir(); err != nil {
			return err
		}
		if err = os.Setenv("XDG_CONFIG_HOME", cfgHomeDir); err != nil {
			return errors.Wrapf(err, "cannot set XDG_CONFIG_HOME")
		}
	}
	path := filepath.Join(cfgHomeDir, "containers")

	// Log to file if not using syslog

	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.MkdirAll(path, 0750); err != nil {
			fmt.Fprintf(os.Stderr, "%v", err)
			return err
		}
	}

	// Update path to include file name
	path = filepath.Join(path, "podman.log")

	// Create the log file if doesn't exist. And append to it if it already exists.
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0640)
	if err != nil {
		// Cannot open log file. Logging to stderr
		fmt.Fprintf(os.Stderr, "%v", err)
		return err
	} else {
		formatter := new(logrus.TextFormatter)
		formatter.FullTimestamp = true
		logrus.SetFormatter(formatter)
		logrus.SetOutput(file)
	}

	// Note this message is only logged if --log-level >= Info!
	logrus.Infof("Logging level set to %s", logrus.GetLevel().String())
	return nil
}
