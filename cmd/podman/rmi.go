package main

import (
	"fmt"
	"os"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/libpod/adapter"
	"github.com/containers/storage"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	rmiCommand     cliconfig.RmiValues
	rmiDescription = "Removes one or more locally stored images."
	_rmiCommand    = &cobra.Command{
		Use:   "rmi",
		Short: "Removes one or more images from local storage",
		Long:  rmiDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			rmiCommand.InputArgs = args
			rmiCommand.GlobalFlags = MainGlobalOpts
			return rmiCmd(&rmiCommand)
		},
		Example: `podman rmi imageID
  podman rmi --force alpine
  podman rmi c4dfb1609ee2 93fd78260bd1 c0ed59d05ff7`,
	}
)

func init() {
	rmiCommand.Command = _rmiCommand
	rmiCommand.SetUsageTemplate(UsageTemplate())
	flags := rmiCommand.Flags()
	flags.BoolVarP(&rmiCommand.All, "all", "a", false, "Remove all images")
	flags.BoolVarP(&rmiCommand.Force, "force", "f", false, "Force Removal of the image")
}

func rmiCmd(c *cliconfig.RmiValues) error {
	var (
		lastError error
		deleted   bool
		deleteErr error
		msg       string
	)

	ctx := getContext()
	removeAll := c.All
	runtime, err := adapter.GetRuntime(&c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	args := c.InputArgs
	if len(args) == 0 && !removeAll {
		return errors.Errorf("image name or ID must be specified")
	}
	if len(args) > 0 && removeAll {
		return errors.Errorf("when using the --all switch, you may not pass any images names or IDs")
	}

	images := args[:]

	removeImage := func(img *adapter.ContainerImage) {
		deleted = true
		msg, deleteErr = runtime.RemoveImage(ctx, img, c.Force)
		if deleteErr != nil {
			if errors.Cause(deleteErr) == storage.ErrImageUsedByContainer {
				fmt.Printf("A container associated with containers/storage, i.e. via Buildah, CRI-O, etc., may be associated with this image: %-12.12s\n", img.ID())
			}
			if lastError != nil {
				fmt.Fprintln(os.Stderr, lastError)
			}
			lastError = deleteErr
		} else {
			fmt.Println(msg)
		}
	}

	if removeAll {
		var imagesToDelete []*adapter.ContainerImage
		imagesToDelete, err = runtime.GetImages()
		if err != nil {
			return errors.Wrapf(err, "unable to query local images")
		}
		lastNumberofImages := 0
		for len(imagesToDelete) > 0 {
			if lastNumberofImages == len(imagesToDelete) {
				return errors.New("unable to delete all images; re-run the rmi command again.")
			}
			for _, i := range imagesToDelete {
				isParent, err := i.IsParent()
				if err != nil {
					return err
				}
				if isParent {
					continue
				}
				removeImage(i)
			}
			lastNumberofImages = len(imagesToDelete)
			imagesToDelete, err = runtime.GetImages()
			if err != nil {
				return err
			}
			// If no images are left to delete or there is just one image left and it cannot be deleted,
			// lets break out and display the error
			if len(imagesToDelete) == 0 || (lastNumberofImages == 1 && lastError != nil) {
				break
			}
		}
	} else {
		// Create image.image objects for deletion from user input.
		// Note that we have to query the storage one-by-one to
		// always get the latest state for each image.  Otherwise, we
		// run inconsistency issues, for instance, with repoTags.
		// See https://github.com/containers/libpod/issues/930 as
		// an exemplary inconsistency issue.
		for _, i := range images {
			newImage, err := runtime.NewImageFromLocal(i)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				continue
			}
			removeImage(newImage)
		}
	}

	// If the user calls remove all and there are none, it should not be a
	// non-zero exit
	if !deleted && removeAll {
		return nil
	}
	// the user tries to remove images that do not exist, that should be a
	// non-zero exit
	if !deleted {
		return errors.Errorf("no valid images to delete")
	}

	return lastError
}
