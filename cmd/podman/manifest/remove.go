package manifest

import (
	"context"
	"fmt"

	"github.com/containers/podman/v3/cmd/podman/common"
	"github.com/containers/podman/v3/cmd/podman/registry"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	removeCmd = &cobra.Command{
		Use:               "remove LIST IMAGE",
		Short:             "Remove an entry from a manifest list or image index",
		Long:              "Removes an image from a manifest list or image index.",
		RunE:              remove,
		ValidArgsFunction: common.AutocompleteImages,
		Example:           `podman manifest remove mylist:v1.11 sha256:15352d97781ffdf357bf3459c037be3efac4133dc9070c2dce7eca7c05c3e736`,
		Args:              cobra.ExactArgs(2),
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: removeCmd,
		Parent:  manifestCmd,
	})
}

func remove(cmd *cobra.Command, args []string) error {
	listImageSpec := args[0]
	instanceSpec := args[1]
	if listImageSpec == "" {
		return errors.Errorf(`invalid image name "%s"`, listImageSpec)
	}
	if instanceSpec == "" {
		return errors.Errorf(`invalid image digest "%s"`, instanceSpec)
	}
	updatedListID, err := registry.ImageEngine().ManifestRemove(context.Background(), args)
	if err != nil {
		return errors.Wrapf(err, "error removing from manifest list %s", listImageSpec)
	}
	fmt.Printf("%s\n", updatedListID)
	return nil
}
