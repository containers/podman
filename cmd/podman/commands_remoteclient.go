// +build remoteclient

package main

import (
	"github.com/spf13/cobra"
)

//import "github.com/urfave/cli"
//
func getAppCommands() []*cobra.Command {
	return []*cobra.Command{}
}

func getImageSubCommands() []*cobra.Command {
	return []*cobra.Command{}
}

func getContainerSubCommands() []*cobra.Command {
	return []*cobra.Command{}
}

func getPodSubCommands() []*cobra.Command {
	return []*cobra.Command{}
}

func getVolumeSubCommands() []*cobra.Command {
	return []*cobra.Command{
		_volumeCreateCommand,
	}
}

func getGenerateSubCommands() []*cobra.Command {
	return []*cobra.Command{}
}

func getPlaySubCommands() []*cobra.Command {
	return []*cobra.Command{}
}

func getTrustSubCommands() []*cobra.Command {
	return []*cobra.Command{}
}

//func getMainAppFlags() []cli.Flag {
//	return []cli.Flag{}
//}
func getSystemSubCommands() []*cobra.Command {
	return []*cobra.Command{}
}
