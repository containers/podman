package images

import (
	"strings"

	"github.com/containers/podman/v3/cmd/podman/registry"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	// podman _images_  Alias for podman image _list_
	imagesCmd = &cobra.Command{
		Use:                   strings.Replace(listCmd.Use, "list", "images", 1),
		Args:                  listCmd.Args,
		Short:                 listCmd.Short,
		Long:                  listCmd.Long,
		RunE:                  listCmd.RunE,
		ValidArgsFunction:     listCmd.ValidArgsFunction,
		Example:               strings.Replace(listCmd.Example, "podman image list", "podman images", -1),
		DisableFlagsInUseLine: true,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: imagesCmd,
	})

	imageListFlagSet(imagesCmd)
}
