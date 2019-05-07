// +build remoteclient

package main

import (
	"github.com/spf13/cobra"
)

const remote = true

func init() {
	//	remote client specific flags can go here.
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
