package secrets

import (
	"context"
	"errors"
	"fmt"

	"github.com/containers/podman/v5/cmd/podman/common"
	"github.com/containers/podman/v5/cmd/podman/registry"
	"github.com/containers/podman/v5/cmd/podman/utils"
	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	rmCmd = &cobra.Command{
		Use:               "rm [options] SECRET [SECRET...]",
		Short:             "Remove one or more secrets",
		RunE:              rm,
		ValidArgsFunction: common.AutocompleteSecrets,
		Example:           "podman secret rm mysecret1 mysecret2",
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: rmCmd,
		Parent:  secretCmd,
	})
	flags := rmCmd.Flags()
	flags.BoolVarP(&rmOptions.All, "all", "a", false, "Remove all secrets")
	flags.BoolVarP(&rmOptions.Ignore, "ignore", "i", false, "Ignore errors when a specified secret is missing")
}

var (
	rmOptions = entities.SecretRmOptions{}
)

func rm(cmd *cobra.Command, args []string) error {
	var (
		errs utils.OutputErrors
	)
	if (len(args) > 0 && rmOptions.All) || (len(args) < 1 && !rmOptions.All) {
		return errors.New("`podman secret rm` requires one argument, or the --all flag")
	}
	responses, err := registry.ContainerEngine().SecretRm(context.Background(), args, rmOptions)
	if err != nil {
		return err
	}
	for _, r := range responses {
		if r.Err == nil {
			fmt.Println(r.ID)
		} else {
			errs = append(errs, r.Err)
		}
	}
	return errs.PrintErrors()
}
