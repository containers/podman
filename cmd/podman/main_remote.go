// +build remoteclient

package main

import (
	"os"

	"github.com/containers/libpod/pkg/rootless"
	"github.com/sirupsen/logrus"
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
	if rootless.IsRootless() {
		became, ret, err := rootless.BecomeRootInUserNS()
		if err != nil {
			logrus.Errorf(err.Error())
			os.Exit(1)
		}
		if became {
			os.Exit(ret)
		}
	}
	return nil
}

func setRLimits() error {
	return nil
}

func setUMask() {}
