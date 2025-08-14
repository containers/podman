package volumes

import (
	"context"
	"fmt"

	"github.com/containers/podman/v5/cmd/podman/common"
	"github.com/containers/podman/v5/cmd/podman/registry"
	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	protectDescription = `Mark or unmark a volume as protected.

Protected volumes are excluded from system prune operations by default.`

	protectCommand = &cobra.Command{
		Use:               "protect [options] VOLUME [VOLUME...]",
		Short:             "Mark or unmark volume as protected",
		Long:              protectDescription,
		RunE:              protect,
		ValidArgsFunction: common.AutocompleteVolumes,
		Example: `podman volume protect myvol
  podman volume protect --unprotect myvol
  podman volume protect vol1 vol2 vol3`,
	}
)

var (
	protectOptions = entities.VolumeProtectOptions{}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: protectCommand,
		Parent:  volumeCmd,
	})
	flags := protectCommand.Flags()
	flags.BoolVar(&protectOptions.Unprotect, "unprotect", false, "Remove protection from volume")
}

func protect(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("must specify at least one volume name")
	}

	responses, err := registry.ContainerEngine().VolumeProtect(context.Background(), args, protectOptions)
	if err != nil {
		return err
	}

	for _, r := range responses {
		if r.Err != nil {
			fmt.Printf("Error protecting volume %s: %v\n", r.Id, r.Err)
		} else {
			if protectOptions.Unprotect {
				fmt.Printf("Volume %s is now unprotected\n", r.Id)
			} else {
				fmt.Printf("Volume %s is now protected\n", r.Id)
			}
		}
	}

	return nil
}
