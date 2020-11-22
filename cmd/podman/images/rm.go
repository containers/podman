package images

import (
	"fmt"

	"github.com/containers/podman/v2/cmd/podman/common"
	"github.com/containers/podman/v2/cmd/podman/registry"
	"github.com/containers/podman/v2/cmd/podman/validate"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/containers/podman/v2/pkg/errorhandling"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	rmDescription = "Removes one or more previously pulled or locally created images."
	rmCmd         = &cobra.Command{
		Use:               "rm [options] IMAGE [IMAGE...]",
		Short:             "Removes one or more images from local storage",
		Long:              rmDescription,
		RunE:              rm,
		Args:              validate.ImagesOrAllArgs,
		ValidArgsFunction: common.AutocompleteImages,
		Example: `podman image rm imageID
  podman image rm --force alpine
  podman image rm c4dfb1609ee2 93fd78260bd1 c0ed59d05ff7`,
	}

	imageOpts = entities.ImageRemoveOptions{}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: rmCmd,
		Parent:  imageCmd,
	})

	imageRemoveFlagSet(rmCmd.Flags())
}

func imageRemoveFlagSet(flags *pflag.FlagSet) {
	flags.BoolVarP(&imageOpts.All, "all", "a", false, "Remove all images")
	flags.BoolVarP(&imageOpts.Force, "force", "f", false, "Force Removal of the image")
}

func rm(cmd *cobra.Command, args []string) error {
	// Note: certain image-removal errors are non-fatal.  Hence, the report
	// might be set even if err != nil.
	report, rmErrors := registry.ImageEngine().Remove(registry.GetContext(), args, imageOpts)
	if report != nil {
		for _, u := range report.Untagged {
			fmt.Println("Untagged: " + u)
		}
		for _, d := range report.Deleted {
			// Make sure an image was deleted (and not just untagged); else print it
			if len(d) > 0 {
				fmt.Println("Deleted: " + d)
			}
		}
		registry.SetExitCode(report.ExitCode)
	}

	return errorhandling.JoinErrors(rmErrors)
}
