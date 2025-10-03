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
	pinDescription = `Mark or unmark a volume as pinned.

Pinned volumes are excluded from system prune and system reset operations.`

	pinCommand = &cobra.Command{
		Use:               "pin [options] VOLUME [VOLUME...]",
		Short:             "Mark or unmark volume as pinned",
		Long:              pinDescription,
		RunE:              pin,
		ValidArgsFunction: common.AutocompleteVolumes,
		Example: `podman volume pin myvol
  podman volume pin --unpin myvol
  podman volume pin vol1 vol2 vol3`,
	}
)

var (
	pinOptions = entities.VolumePinOptions{}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: pinCommand,
		Parent:  volumeCmd,
	})
	flags := pinCommand.Flags()
	flags.BoolVar(&pinOptions.Unpin, "unpin", false, "Remove pinning from volume")
}

func pin(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("must specify at least one volume name")
	}

	responses, err := registry.ContainerEngine().VolumePin(context.Background(), args, pinOptions)
	if err != nil {
		return err
	}

	for _, r := range responses {
		if r.Err != nil {
			fmt.Printf("Error pinning volume %s: %v\n", r.Id, r.Err)
		} else {
			if pinOptions.Unpin {
				fmt.Printf("Volume %s is now unpinned\n", r.Id)
			} else {
				fmt.Printf("Volume %s is now pinned\n", r.Id)
			}
		}
	}

	return nil
}
