package volumes

import (
	"fmt"

	"github.com/containers/podman/v5/cmd/podman/common"
	"github.com/containers/podman/v5/cmd/podman/registry"
	"github.com/containers/podman/v5/cmd/podman/utils"
	"github.com/spf13/cobra"
)

var (
	volumeMountDescription = `Mount a volume and return the mountpoint`
	volumeMountCommand     = &cobra.Command{
		Annotations: map[string]string{
			registry.UnshareNSRequired: "",
			registry.ParentNSRequired:  "",
			registry.EngineMode:        registry.ABIMode,
		},
		Use:               "mount NAME",
		Short:             "Mount volume",
		Long:              volumeMountDescription,
		RunE:              volumeMount,
		Example:           `podman volume mount myvol`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: common.AutocompleteVolumes,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: volumeMountCommand,
		Parent:  volumeCmd,
	})
}

func volumeMount(cmd *cobra.Command, args []string) error {
	var errs utils.OutputErrors
	reports, err := registry.ContainerEngine().VolumeMount(registry.GetContext(), args)
	if err != nil {
		return err
	}
	for _, r := range reports {
		if r.Err == nil {
			fmt.Println(r.Path)
			continue
		}
		errs = append(errs, r.Err)
	}
	return errs.PrintErrors()
}
