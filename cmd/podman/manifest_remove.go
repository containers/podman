package main

import (
	"fmt"

	"github.com/containers/buildah/manifests"
	"github.com/containers/buildah/pkg/parse"
	"github.com/containers/buildah/util"
	"github.com/containers/libpod/cmd/podman/cliconfig"
	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	manifestRemoveCommand     cliconfig.ManifestRemoveValues
	manifestRemoveDescription = `removes an image from a manifest list or image index`
	_manifestRemoveCommand    = &cobra.Command{
		Use:   "remove [image] [hash]",
		Short: "manifest remove",
		Long:  manifestRemoveDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestRemoveCommand.InputArgs = args
			manifestRemoveCommand.GlobalFlags = MainGlobalOpts
			manifestRemoveCommand.Remote = remoteclient
			return manifestRemoveCmd(&manifestRemoveCommand)
		},
		Example: `podman manifest remove mylist:v1.11 sha256:15352d97781ffdf357bf3459c037be3efac4133dc9070c2dce7eca7c05c3e736`,
		Args:    cobra.MinimumNArgs(2),
	}
)

func init() {
	manifestRemoveCommand.Command = _manifestRemoveCommand
	manifestRemoveCommand.SetUsageTemplate(HelpTemplate())
	manifestRemoveCommand.SetUsageTemplate(UsageTemplate())
}

func manifestRemoveCmd(c *cliconfig.ManifestRemoveValues) error {
	listImageSpec := ""
	var instanceDigest digest.Digest
	args := c.InputArgs
	switch len(args) {
	case 0, 1:
		return errors.New("At least a list image and one or more instance digests must be specified")
	case 2:
		listImageSpec = args[0]
		if listImageSpec == "" {
			return errors.Errorf(`Invalid image name "%s"`, args[0])
		}
		instanceSpec := args[1]
		if instanceSpec == "" {
			return errors.Errorf(`Invalid instance "%s"`, args[1])
		}
		d, err := digest.Parse(instanceSpec)
		if err != nil {
			return errors.Errorf(`Invalid instance "%s": %v`, args[1], err)
		}
		instanceDigest = d
	default:
		return errors.New("At least two arguments are necessary: list and digest of instance to remove from list")
	}

	store, err := getStore(c)
	if err != nil {
		return err
	}

	systemContext, err := parse.SystemContextFromOptions(c.Command)
	if err != nil {
		return errors.Wrapf(err, "error building system context")
	}

	_, listImage, err := util.FindImage(store, "", systemContext, listImageSpec)
	if err != nil {
		return err
	}

	_, list, err := manifests.LoadFromImage(store, listImage.ID)
	if err != nil {
		return err
	}

	err = list.Remove(instanceDigest)
	if err != nil {
		return err
	}

	updatedListID, err := list.SaveToImage(store, listImage.ID, nil, "")
	if err == nil {
		fmt.Printf("%s: %s\n", updatedListID, instanceDigest.String())
	}

	return nil
}
