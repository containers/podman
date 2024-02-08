package volumes

import (
	"fmt"

	"github.com/containers/podman/v5/cmd/podman/common"
	"github.com/containers/podman/v5/cmd/podman/registry"
	"github.com/containers/podman/v5/cmd/podman/utils"
	"github.com/spf13/cobra"
)

var (
	volumeUnmountDescription = `Unmount a volume`
	volumeUnmountCommand     = &cobra.Command{
		Annotations:       map[string]string{registry.EngineMode: registry.ABIMode},
		Use:               "unmount NAME",
		Short:             "Unmount volume",
		Long:              volumeUnmountDescription,
		RunE:              volumeUnmount,
		Example:           `podman volume unmount myvol`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: common.AutocompleteVolumes,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: volumeUnmountCommand,
		Parent:  volumeCmd,
	})
}

func volumeUnmount(cmd *cobra.Command, args []string) error {
	var errs utils.OutputErrors
	reports, err := registry.ContainerEngine().VolumeUnmount(registry.GetContext(), args)
	if err != nil {
		return err
	}
	for _, r := range reports {
		if r.Err == nil {
			fmt.Println(r.Id)
		} else {
			errs = append(errs, r.Err)
		}
	}
	return errs.PrintErrors()
}
