// +build !remoteclient,!linux

package main

// The ONLY purpose of this file is to allow the subpackage to compile. Don’t expect anything
// to work.

import (
	"syscall"

	"github.com/spf13/cobra"
)

const remote = false

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

func setUMask() {
	// Be sure we can create directories with 0755 mode.
	syscall.Umask(0022)
}

// checkInput can be used to verify any of the globalopt values
func checkInput() error {
	return nil
}
