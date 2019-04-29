// +build remoteclient

package main

import (
	"github.com/spf13/cobra"
)

const remote = true

func init() {
	rootCmd.PersistentFlags().StringVar(&MainGlobalOpts.RemoteUserName, "username", "", "username on the remote host")
	rootCmd.PersistentFlags().StringVar(&MainGlobalOpts.RemoteHost, "remote-host", "", "remote host")
	// TODO maybe we allow the altering of this for bridge connections?
	//rootCmd.PersistentFlags().StringVar(&MainGlobalOpts.VarlinkAddress, "varlink-address", adapter.DefaultAddress, "address of the varlink socket")
	rootCmd.PersistentFlags().StringVar(&MainGlobalOpts.LogLevel, "log-level", "error", "Log messages above specified level: debug, info, warn, error, fatal or panic")
}

func setSyslog() error {
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
