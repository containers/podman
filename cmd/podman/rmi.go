package main

import (
	"fmt"
	"os"

	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/libpod/image"
	"github.com/containers/storage"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

var (
	rmiDescription = "Removes one or more locally stored images."
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
		Usage:       "Removes one or more images from local storage",
		Description: rmiDescription,
		Action:      rmiCmd,
		ArgsUsage:   "IMAGE-NAME-OR-ID [...]",
		Flags:       rmiFlags,
		UseShortOptionHandling: true,
		OnUsageError:           usageErrorHandler,
	}
	rmImageCommand = cli.Command{
		Name:        "rm",
		Usage:       "removes one or more images from local storage",
		Description: rmiDescription,
		Action:      rmiCmd,
		ArgsUsage:   "IMAGE-NAME-OR-ID [...]",
		Flags:       rmiFlags,
		UseShortOptionHandling: true,
		OnUsageError:           usageErrorHandler,
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
	var deleted bool

	removeImage := func(img *image.Image) {
		deleted = true
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

	if removeAll {
		var imagesToDelete []*image.Image
		imagesToDelete, err = runtime.ImageRuntime().GetImages()
		if err != nil {
			return errors.Wrapf(err, "unable to query local images")
		}
		for _, i := range imagesToDelete {
			removeImage(i)
		}
	} else {
		// Create image.image objects for deletion from user input.
		// Note that we have to query the storage one-by-one to
		// always get the latest state for each image.  Otherwise, we
		// run inconsistency issues, for instance, with repoTags.
		// See https://github.com/containers/libpod/issues/930 as
		// an exemplary inconsistency issue.
		for _, i := range images {
			newImage, err := runtime.ImageRuntime().NewFromLocal(i)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				continue
			}
			removeImage(newImage)
		}
	}

	if !deleted {
		return errors.Errorf("no valid images to delete")
	}

	return lastError
}
