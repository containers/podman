package main

import (
	"os"

	"github.com/containers/libpod/libpod/adapter"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/pkg/errors"
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
		Name:         "export",
		Usage:        "Export container's filesystem contents as a tar archive",
		Description:  exportDescription,
		Flags:        sortFlags(exportFlags),
		Action:       exportCmd,
		ArgsUsage:    "CONTAINER",
		OnUsageError: usageErrorHandler,
	}
)

// exportCmd saves a container to a tarball on disk
func exportCmd(c *cli.Context) error {
	if err := validateFlags(c, exportFlags); err != nil {
		return err
	}
	if os.Geteuid() != 0 {
		rootless.SetSkipStorageSetup(true)
	}

	runtime, err := adapter.GetRuntime(c)
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
	if runtime.Remote && (output == "/dev/stdout" || len(output) == 0) {
		return errors.New("remote client usage must specify an output file (-o)")
	}
	if output == "/dev/stdout" {
		file := os.Stdout
		if logrus.IsTerminal(file) {
			return errors.Errorf("refusing to export to terminal. Use -o flag or redirect")
		}
	}

	if err := validateFileName(output); err != nil {
		return err
	}
	return runtime.Export(args[0], c.String("output"))
}
