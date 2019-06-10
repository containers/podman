package main

import (
	"fmt"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/pkg/adapter"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	pruneImagesCommand     cliconfig.PruneImagesValues
	pruneImagesDescription = `Removes all unnamed images from local storage.

  If an image is not being used by a container, it will be removed from the system.`
	_pruneImagesCommand = &cobra.Command{
		Use:   "prune",
		Args:  noSubArgs,
		Short: "Remove unused images",
		Long:  pruneImagesDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			pruneImagesCommand.InputArgs = args
			pruneImagesCommand.GlobalFlags = MainGlobalOpts
			pruneImagesCommand.Remote = remoteclient
			return pruneImagesCmd(&pruneImagesCommand)
		},
	}
)

func init() {
	pruneImagesCommand.Command = _pruneImagesCommand
	pruneImagesCommand.SetHelpTemplate(HelpTemplate())
	pruneImagesCommand.SetUsageTemplate(UsageTemplate())
	flags := pruneImagesCommand.Flags()
	flags.BoolVarP(&pruneImagesCommand.All, "all", "a", false, "Remove all unused images, not just dangling ones")
}

func pruneImagesCmd(c *cliconfig.PruneImagesValues) error {
	runtime, err := adapter.GetRuntime(getContext(), &c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	// Call prune; if any cids are returned, print them and then
	// return err in case an error also came up
	pruneCids, err := runtime.PruneImages(getContext(), c.All)
	if len(pruneCids) > 0 {
		for _, cid := range pruneCids {
			fmt.Println(cid)
		}
	}
	return err
}
