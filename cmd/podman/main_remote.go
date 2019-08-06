// +build remoteclient

package main

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"

	"github.com/containers/libpod/pkg/util"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

const remote = true

func init() {
	var username string
	if curruser, err := user.Current(); err == nil {
		username = curruser.Username
	}
	rootCmd.PersistentFlags().StringVar(&MainGlobalOpts.ConnectionName, "connection", "", "remote connection name")
	rootCmd.PersistentFlags().StringVar(&MainGlobalOpts.RemoteConfigFilePath, "remote-config-path", "", "alternate path for configuration file")
	rootCmd.PersistentFlags().StringVar(&MainGlobalOpts.RemoteUserName, "username", username, "username on the remote host")
	rootCmd.PersistentFlags().StringVar(&MainGlobalOpts.RemoteHost, "remote-host", "", "remote host")
	// TODO maybe we allow the altering of this for bridge connections?
	// rootCmd.PersistentFlags().StringVar(&MainGlobalOpts.VarlinkAddress, "varlink-address", adapter.DefaultAddress, "address of the varlink socket")
	rootCmd.PersistentFlags().StringVar(&MainGlobalOpts.LogLevel, "log-level", "error", "Log messages above specified level: debug, info, warn, error, fatal or panic. Logged to ~/.config/containers/podman.log")
	rootCmd.PersistentFlags().BoolVar(&MainGlobalOpts.Syslog, "syslog", false, "Output logging information to syslog as well as the console")
}

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

func profileOn(cmd *cobra.Command) error {
	return nil
}

func profileOff(cmd *cobra.Command) error {
	return nil
}

func setupRootless(cmd *cobra.Command, args []string) error {
	return nil
}

func setRLimits() error {
	return nil
}

func setUMask() {}
