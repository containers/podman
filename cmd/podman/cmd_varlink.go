// +build varlink

package main

import (
	"github.com/urfave/cli"
)

// getOptionalCommands returns an array of commands to be added to podman
// when varlink is not specifically enabled, we return an empty array for now.
func getOptionalCommands() []cli.Command {
	return []cli.Command{varlinkCommand}
}
