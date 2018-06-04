package main

import (
	"fmt"
	"os"

	"github.com/containers/storage"
	"github.com/pkg/errors"
	"github.com/projectatomic/libpod/cmd/podman/libpodruntime"
	"github.com/projectatomic/libpod/libpod/image"
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
		UseShortOptionHandling: true,
	}
)

func rmiCmd(c *cli.Context) error {
	ctx := getContext()
	if err := validateFlags(c, rmiFlags); err != nil {
		return err
	}
	removeAll := c.Bool("all")
	runtime, err := libpodruntime.GetRuntime(c)
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

	images := args[:]
	var lastError error
	var imagesToDelete []*image.Image
	if removeAll {
		imagesToDelete, err = runtime.ImageRuntime().GetImages()
		if err != nil {
			return errors.Wrapf(err, "unable to query local images")
		}
	} else {
		// Create image.image objects for deletion from user input
		for _, i := range images {
			newImage, err := runtime.ImageRuntime().NewFromLocal(i)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				continue
			}
			imagesToDelete = append(imagesToDelete, newImage)
		}
	}
	if len(imagesToDelete) == 0 {
		return errors.Errorf("no valid images to delete")
	}
	for _, img := range imagesToDelete {
		msg, err := runtime.RemoveImage(ctx, img, c.Bool("force"))
		if err != nil {
			if errors.Cause(err) == storage.ErrImageUsedByContainer {
				fmt.Printf("A container associated with containers/storage, i.e. via Buildah, CRI-O, etc., may be associated with this image: %-12.12s\n", img.ID())
			}
			if lastError != nil {
				fmt.Fprintln(os.Stderr, lastError)
			}
			lastError = err
		} else {
			fmt.Println(msg)
		}
	}
	return lastError
}
