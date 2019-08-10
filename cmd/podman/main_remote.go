// +build remoteclient

package main

import (
	"os/user"

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
