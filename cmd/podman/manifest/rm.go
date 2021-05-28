package manifest

import (
	"context"
	"fmt"

	"github.com/containers/podman/v3/cmd/podman/common"
	"github.com/containers/podman/v3/cmd/podman/registry"
	"github.com/containers/podman/v3/pkg/errorhandling"
	"github.com/spf13/cobra"
)

var (
	rmCmd = &cobra.Command{
		Use:               "rm LIST",
		Short:             "Remove manifest list or image index from local storage",
		Long:              "Remove manifest list or image index from local storage.",
		RunE:              rm,
		ValidArgsFunction: common.AutocompleteImages,
		Example:           `podman manifest rm mylist:v1.11`,
		Args:              cobra.ExactArgs(1),
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: rmCmd,
		Parent:  manifestCmd,
	})
}

func rm(cmd *cobra.Command, args []string) error {
	report, rmErrors := registry.ImageEngine().ManifestRm(context.Background(), args)
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
