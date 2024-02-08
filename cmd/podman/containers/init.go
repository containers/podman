package containers

import (
	"fmt"

	"github.com/containers/podman/v5/cmd/podman/common"
	"github.com/containers/podman/v5/cmd/podman/registry"
	"github.com/containers/podman/v5/cmd/podman/utils"
	"github.com/containers/podman/v5/cmd/podman/validate"
	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	initDescription = `Initialize one or more containers, creating the OCI spec and mounts for inspection. Container names or IDs can be used.`

	initCommand = &cobra.Command{
		Use:   "init [options] CONTAINER [CONTAINER...]",
		Short: "Initialize one or more containers",
		Long:  initDescription,
		RunE:  initContainer,
		Args: func(cmd *cobra.Command, args []string) error {
			return validate.CheckAllLatestAndIDFile(cmd, args, false, "")
		},
		ValidArgsFunction: common.AutocompleteContainersCreated,
		Example: `podman init 3c45ef19d893
  podman init test1`,
	}

	containerInitCommand = &cobra.Command{
		Use:               initCommand.Use,
		Short:             initCommand.Short,
		Long:              initCommand.Long,
		RunE:              initCommand.RunE,
		Args:              initCommand.Args,
		ValidArgsFunction: initCommand.ValidArgsFunction,
		Example: `podman container init 3c45ef19d893
  podman container init test1`,
	}
)

var (
	initOptions entities.ContainerInitOptions
)

func initFlags(flags *pflag.FlagSet) {
	flags.BoolVarP(&initOptions.All, "all", "a", false, "Initialize all containers")
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: initCommand,
	})
	flags := initCommand.Flags()
	initFlags(flags)
	validate.AddLatestFlag(initCommand, &initOptions.Latest)

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Parent:  containerCmd,
		Command: containerInitCommand,
	})
	containerInitFlags := containerInitCommand.Flags()
	initFlags(containerInitFlags)
	validate.AddLatestFlag(containerInitCommand, &initOptions.Latest)
}

func initContainer(cmd *cobra.Command, args []string) error {
	var errs utils.OutputErrors
	args = utils.RemoveSlash(args)
	report, err := registry.ContainerEngine().ContainerInit(registry.GetContext(), args, initOptions)
	if err != nil {
		return err
	}
	for _, r := range report {
		switch {
		case r.Err != nil:
			errs = append(errs, r.Err)
		case r.RawInput != "":
			fmt.Println(r.RawInput)
		default:
			fmt.Println(r.Id)
		}
	}
	return errs.PrintErrors()
}
