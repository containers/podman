package quadlet

import (
	"errors"
	"fmt"

	"github.com/containers/podman/v5/cmd/podman/registry"
	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	quadletRmDescription = `Remove one or more installed Quadlets from the current user`

	quadletRmCmd = &cobra.Command{
		Use:   "rm [options] QUADLET [QUADLET...]",
		Short: "Remove Quadlets",
		Long:  quadletRmDescription,
		RunE:  rm,
		// TODO: Arg validation plus completion
		Example: `podman quadlet rm test.container
podman quadlet rm --force mysql.container
podman quadlet rm --all --reload-systemd=false`,
	}

	removeOptions entities.QuadletRemoveOptions
)

func rmFlags(cmd *cobra.Command) {
	flags := cmd.Flags()

	flags.BoolVarP(&removeOptions.Force, "force", "f", false, "Remove running quadlets")
	flags.BoolVarP(&removeOptions.All, "all", "a", false, "Remove all Quadlets for the current user")
	flags.BoolVarP(&removeOptions.Ignore, "ignore", "i", false, "Do not error for Quadlets that do not exist")
	flags.BoolVar(&removeOptions.ReloadSystemd, "reload-systemd", false, "Reload systemd after removal")
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: quadletRmCmd,
		Parent:  quadletCmd,
	})
	rmFlags(quadletRmCmd)
}

func rm(cmd *cobra.Command, args []string) error {
	removeReport, err := registry.ContainerEngine().QuadletRemove(registry.Context(), args, removeOptions)
	// We can get a report back even if err != nil if systemd reload failed
	if removeReport != nil {
		for _, rq := range removeReport.Removed {
			fmt.Printf("%s\n", rq)
		}
		for quadlet, quadletErr := range removeReport.Errors {
			logrus.Errorf("Unable to remove Quadlet %s: %v", quadlet, quadletErr)
		}

		if err == nil && len(removeReport.Errors) > 0 {
			return errors.New("some quadlets could not be removed")
		}
	}

	if err != nil {
		return err
	}

	return nil
}
