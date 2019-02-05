package main

import (
	"fmt"

	"github.com/containers/libpod/libpod/adapter"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

var (
	pruneImagesDescription = `
	podman image prune

	Removes all unnamed images from local storage
`
	pruneImageFlags = []cli.Flag{
		cli.BoolFlag{
			Name:  "all, a",
			Usage: "remove all unused images, not just dangling ones",
		},
	}
	pruneImagesCommand = cli.Command{
		Name:         "prune",
		Usage:        "Remove unused images",
		Description:  pruneImagesDescription,
		Action:       pruneImagesCmd,
		OnUsageError: usageErrorHandler,
		Flags:        pruneImageFlags,
	}
)

func pruneImagesCmd(c *cli.Context) error {
	runtime, err := adapter.GetRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	// Call prune; if any cids are returned, print them and then
	// return err in case an error also came up
	pruneCids, err := runtime.PruneImages(c.Bool("all"))
	if len(pruneCids) > 0 {
		for _, cid := range pruneCids {
			fmt.Println(cid)
		}
	}
	return err
}
