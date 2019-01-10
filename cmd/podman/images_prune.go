package main

import (
	"fmt"
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

var (
	pruneImagesDescription = `
	podman image prune

	Removes all unnamed images from local storage
`

	pruneImagesCommand = cli.Command{
		Name:         "prune",
		Usage:        "Remove unused images",
		Description:  pruneImagesDescription,
		Action:       pruneImagesCmd,
		OnUsageError: usageErrorHandler,
	}
)

func pruneImagesCmd(c *cli.Context) error {
	runtime, err := libpodruntime.GetRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	pruneImages, err := runtime.ImageRuntime().GetPruneImages()
	if err != nil {
		return err
	}

	for _, i := range pruneImages {
		if err := i.Remove(true); err != nil {
			return errors.Wrapf(err, "failed to remove %s", i.ID())
		}
		fmt.Println(i.ID())
	}
	return nil
}
