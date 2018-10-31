package main

import (
	"fmt"
	"time"

	"github.com/containers/libpod/libpod"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

// versionCmd gets and prints version info for version command
func versionCmd(c *cli.Context) error {
	output, err := libpod.GetVersion()
	if err != nil {
		errors.Wrapf(err, "unable to determine version")
	}
	fmt.Println("Version:      ", output.Version)
	fmt.Println("Go Version:   ", output.GoVersion)
	if output.GitCommit != "" {
		fmt.Println("Git Commit:   ", output.GitCommit)
	}
	// Prints out the build time in readable format
	if output.Built != 0 {
		fmt.Println("Built:        ", time.Unix(output.Built, 0).Format(time.ANSIC))
	}

	fmt.Println("OS/Arch:      ", output.OsArch)
	return nil
}

// Cli command to print out the full version of podman
var versionCommand = cli.Command{
	Name:   "version",
	Usage:  "Display the PODMAN Version Information",
	Action: versionCmd,
}
