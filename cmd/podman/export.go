package main

import (
	"os"

	"github.com/pkg/errors"
	"github.com/projectatomic/libpod/cmd/podman/libpodruntime"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var (
	exportFlags = []cli.Flag{
		cli.StringFlag{
			Name:  "output, o",
			Usage: "Write to a file, default is STDOUT",
			Value: "/dev/stdout",
		},
	}
	exportDescription = "Exports container's filesystem contents as a tar archive" +
		" and saves it on the local machine."
	exportCommand = cli.Command{
		Name:        "export",
		Usage:       "Export container's filesystem contents as a tar archive",
		Description: exportDescription,
		Flags:       exportFlags,
		Action:      exportCmd,
		ArgsUsage:   "CONTAINER",
	}
)

// exportCmd saves a container to a tarball on disk
func exportCmd(c *cli.Context) error {
	if err := validateFlags(c, exportFlags); err != nil {
		return err
	}

	runtime, err := libpodruntime.GetRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	args := c.Args()
	if len(args) == 0 {
		return errors.Errorf("container id must be specified")
	}
	if len(args) > 1 {
		return errors.Errorf("too many arguments given, need 1 at most.")
	}

	output := c.String("output")
	if output == "/dev/stdout" {
		file := os.Stdout
		if logrus.IsTerminal(file) {
			return errors.Errorf("refusing to export to terminal. Use -o flag or redirect")
		}
	}

	ctr, err := runtime.LookupContainer(args[0])
	if err != nil {
		return errors.Wrapf(err, "error looking up container %q", args[0])
	}

	return ctr.Export(output)
}
