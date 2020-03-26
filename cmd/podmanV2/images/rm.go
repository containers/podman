package images

import (
	"fmt"
	"os"

	"github.com/containers/libpod/cmd/podmanV2/registry"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	rmDescription = "Removes one or more previously pulled or locally created images."
	rmCmd         = &cobra.Command{
		Use:     "rm [flags] IMAGE [IMAGE...]",
		Short:   "Removes one or more images from local storage",
		Long:    rmDescription,
		PreRunE: preRunE,
		RunE:    rm,
		Example: `podman image rm imageID
  podman image rm --force alpine
  podman image rm c4dfb1609ee2 93fd78260bd1 c0ed59d05ff7`,
	}

	imageOpts = entities.ImageDeleteOptions{}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: rmCmd,
		Parent:  imageCmd,
	})

	flags := rmCmd.Flags()
	flags.BoolVarP(&imageOpts.All, "all", "a", false, "Remove all images")
	flags.BoolVarP(&imageOpts.Force, "force", "f", false, "Force Removal of the image")
}

func rm(cmd *cobra.Command, args []string) error {

	if len(args) < 1 && !imageOpts.All {
		return errors.Errorf("image name or ID must be specified")
	}
	if len(args) > 0 && imageOpts.All {
		return errors.Errorf("when using the --all switch, you may not pass any images names or IDs")
	}

	report, err := registry.ImageEngine().Delete(registry.GetContext(), args, imageOpts)
	if err != nil {
		switch {
		case report != nil && report.ImageNotFound != nil:
			fmt.Fprintln(os.Stderr, err.Error())
			registry.SetExitCode(2)
		case report != nil && report.ImageInUse != nil:
			fmt.Fprintln(os.Stderr, err.Error())
		default:
			return err
		}
	}

	for _, u := range report.Untagged {
		fmt.Println("Untagged: " + u)
	}
	for _, d := range report.Deleted {
		fmt.Println("Deleted: " + d)
	}
	return nil
}
