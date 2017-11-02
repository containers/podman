package main

import (
	"io"
	"os"

	"github.com/pkg/errors"
	"github.com/projectatomic/libpod/libpod"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var (
	saveFlags = []cli.Flag{
		cli.StringFlag{
			Name:  "output, o",
			Usage: "Write to a file, default is STDOUT",
			Value: "/dev/stdout",
		},
		cli.BoolFlag{
			Name:  "quiet, q",
			Usage: "Suppress the output",
		},
		cli.StringFlag{
			Name:  "format",
			Usage: "Save image to oci-archive",
		},
	}
	saveDescription = `
	Save an image to docker-archive or oci-archive on the local machine.
	Default is docker-archive`

	saveCommand = cli.Command{
		Name:        "save",
		Usage:       "Save image to an archive",
		Description: saveDescription,
		Flags:       saveFlags,
		Action:      saveCmd,
		ArgsUsage:   "",
	}
)

// saveCmd saves the image to either docker-archive or oci
func saveCmd(c *cli.Context) error {
	args := c.Args()
	if len(args) == 0 {
		return errors.Errorf("need at least 1 argument")
	}
	if err := validateFlags(c, saveFlags); err != nil {
		return err
	}

	runtime, err := getRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "could not create runtime")
	}
	defer runtime.Shutdown(false)

	var writer io.Writer
	if !c.Bool("quiet") {
		writer = os.Stdout
	}

	output := c.String("output")
	if output == "/dev/stdout" {
		fi := os.Stdout
		if logrus.IsTerminal(fi) {
			return errors.Errorf("refusing to save to terminal. Use -o flag or redirect")
		}
	}

	var dst string
	switch c.String("format") {
	case libpod.OCIArchive:
		dst = libpod.OCIArchive + ":" + output
	case libpod.DockerArchive:
		fallthrough
	case "":
		dst = libpod.DockerArchive + ":" + output
	default:
		return errors.Errorf("unknown format option %q", c.String("format"))
	}

	saveOpts := libpod.CopyOptions{
		SignaturePolicyPath: "",
		Writer:              writer,
	}

	// only one image is supported for now
	// future pull requests will fix this
	for _, image := range args {
		dest := dst + ":" + image
		if err := runtime.PushImage(image, dest, saveOpts); err != nil {
			return errors.Wrapf(err, "unable to save %q", image)
		}
	}
	return nil
}
