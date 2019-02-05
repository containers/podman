package main

import (
	"fmt"

	"github.com/containers/libpod/libpod/adapter"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

var (
	importFlags = []cli.Flag{
		cli.StringSliceFlag{
			Name:  "change, c",
			Usage: "Apply the following possible instructions to the created image (default []): CMD | ENTRYPOINT | ENV | EXPOSE | LABEL | STOPSIGNAL | USER | VOLUME | WORKDIR",
		},
		cli.StringFlag{
			Name:  "message, m",
			Usage: "Set commit message for imported image",
		},
		cli.BoolFlag{
			Name:  "quiet, q",
			Usage: "Suppress output",
		},
	}
	importDescription = `Create a container image from the contents of the specified tarball (.tar, .tar.gz, .tgz, .bzip, .tar.xz, .txz).
	 Note remote tar balls can be specified, via web address.
	 Optionally tag the image. You can specify the instructions using the --change option.
	`
	importCommand = cli.Command{
		Name:         "import",
		Usage:        "Import a tarball to create a filesystem image",
		Description:  importDescription,
		Flags:        sortFlags(importFlags),
		Action:       importCmd,
		ArgsUsage:    "TARBALL [REFERENCE]",
		OnUsageError: usageErrorHandler,
	}
)

func importCmd(c *cli.Context) error {
	if err := validateFlags(c, importFlags); err != nil {
		return err
	}

	runtime, err := adapter.GetRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	var (
		source    string
		reference string
	)

	args := c.Args()
	switch len(args) {
	case 0:
		return errors.Errorf("need to give the path to the tarball, or must specify a tarball of '-' for stdin")
	case 1:
		source = args[0]
	case 2:
		source = args[0]
		reference = args[1]
	default:
		return errors.Errorf("too many arguments. Usage TARBALL [REFERENCE]")
	}

	if err := validateFileName(source); err != nil {
		return err
	}

	quiet := c.Bool("quiet")
	if runtime.Remote {
		quiet = false
	}
	iid, err := runtime.Import(getContext(), source, reference, c.StringSlice("change"), c.String("message"), quiet)
	if err == nil {
		fmt.Println(iid)
	}
	return err
}
