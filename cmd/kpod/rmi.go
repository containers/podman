package main

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/projectatomic/libpod/libpod"
	"github.com/urfave/cli"
)

var (
	rmiDescription = "removes one or more locally stored images."
	rmiFlags       = []cli.Flag{
		cli.BoolFlag{
			Name:  "all, a",
			Usage: "remove all images",
		},
		cli.BoolFlag{
			Name:  "force, f",
			Usage: "force removal of the image",
		},
	}
	rmiCommand = cli.Command{
		Name:        "rmi",
		Usage:       "removes one or more images from local storage",
		Description: rmiDescription,
		Action:      rmiCmd,
		ArgsUsage:   "IMAGE-NAME-OR-ID [...]",
		Flags:       rmiFlags,
	}
)

func rmiCmd(c *cli.Context) error {
	if err := validateFlags(c, rmiFlags); err != nil {
		return err
	}
	removeAll := c.Bool("all")
	runtime, err := getRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	args := c.Args()
	if len(args) == 0 && !removeAll {
		return errors.Errorf("image name or ID must be specified")
	}
	if len(args) > 0 && removeAll {
		return errors.Errorf("when using the --all switch, you may not pass any images names or IDs")
	}
	imagesToDelete := args[:]
	if removeAll {
		localImages, err := runtime.GetImages(&libpod.ImageFilterParams{})
		if err != nil {
			return errors.Wrapf(err, "unable to query local images")
		}
		for _, image := range localImages {
			imagesToDelete = append(imagesToDelete, image.ID)
		}
	}

	for _, arg := range imagesToDelete {
		image, err := runtime.GetImage(arg)
		if err != nil {
			return errors.Wrapf(err, "could not get image %q", arg)
		}
		id, err := runtime.RemoveImage(image, c.Bool("force"))
		if err != nil {
			return errors.Wrapf(err, "error removing image %q", id)
		}
		fmt.Printf("%s\n", id)
	}
	return nil
}
