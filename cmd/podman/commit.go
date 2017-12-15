package main

import (
	"github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/projectatomic/libpod/libpod"
	"github.com/urfave/cli"
)

var (
	commitFlags = []cli.Flag{
		cli.StringSliceFlag{
			Name:  "change, c",
			Usage: "Apply the following possible instructions to the created image (default []): CMD | ENTRYPOINT | ENV | EXPOSE | LABEL | STOPSIGNAL | USER | VOLUME | WORKDIR",
		},
		cli.StringFlag{
			Name:  "message, m",
			Usage: "Set commit message for imported image",
		},
		cli.StringFlag{
			Name:  "author, a",
			Usage: "Set the author for the image comitted",
		},
		cli.BoolTFlag{
			Name:  "pause, p",
			Usage: "Pause container during commit",
		},
	}
	commitDescription = `Create an image from a container's changes.
	 Optionally tag the image created, set the author with the --author flag,
	 set the commit message with the --message flag,
	 and make changes to the instructions with the --change flag.`
	commitCommand = cli.Command{
		Name:        "commit",
		Usage:       "Create new image based on the changed container",
		Description: commitDescription,
		Flags:       commitFlags,
		Action:      commitCmd,
		ArgsUsage:   "CONTAINER [REPOSITORY[:TAG]]",
	}
)

func commitCmd(c *cli.Context) error {
	if err := validateFlags(c, commitFlags); err != nil {
		return err
	}

	runtime, err := getRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	var opts libpod.CopyOptions
	var container string
	args := c.Args()
	switch len(args) {
	case 0:
		return errors.Errorf("need to give container name or id")
	case 1:
		container = args[0]
	case 2:
		container = args[0]
		opts.Reference = args[1]
	default:
		return errors.Errorf("too many arguments. Usage CONTAINER [REFERENCE]")
	}

	changes := v1.ImageConfig{}
	if c.IsSet("change") {
		changes, err = getImageConfig(c.StringSlice("change"))
		if err != nil {
			return errors.Wrapf(err, "error adding config changes to image %q", container)
		}
	}

	history := []v1.History{
		{Comment: c.String("message")},
	}

	config := v1.Image{
		Config:  changes,
		History: history,
		Author:  c.String("author"),
	}
	opts.ImageConfig = config

	ctr, err := runtime.LookupContainer(container)
	if err != nil {
		return errors.Wrapf(err, "error looking up container %q", container)
	}
	return ctr.Commit(c.BoolT("pause"), opts)
}
