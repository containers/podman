package containers

import (
	"github.com/containers/podman/v2/cmd/podman/common"
	"github.com/containers/podman/v2/cmd/podman/registry"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	cpDescription = `Command copies the contents of SRC_PATH to the DEST_PATH.

  You can copy from the container's file system to the local machine or the reverse, from the local filesystem to the container. If "-" is specified for either the SRC_PATH or DEST_PATH, you can also stream a tar archive from STDIN or to STDOUT. The CONTAINER can be a running or stopped container.  The SRC_PATH or DEST_PATH can be a file or directory.
`
	cpCommand = &cobra.Command{
		Use:               "cp [options] [CONTAINER:]SRC_PATH [CONTAINER:]DEST_PATH",
		Short:             "Copy files/folders between a container and the local filesystem",
		Long:              cpDescription,
		Args:              cobra.ExactArgs(2),
		RunE:              cp,
		ValidArgsFunction: common.AutocompleteCpCommand,
		Example:           "podman cp [CONTAINER:]SRC_PATH [CONTAINER:]DEST_PATH",
	}

	containerCpCommand = &cobra.Command{
		Use:               cpCommand.Use,
		Short:             cpCommand.Short,
		Long:              cpCommand.Long,
		Args:              cpCommand.Args,
		RunE:              cpCommand.RunE,
		ValidArgsFunction: cpCommand.ValidArgsFunction,
		Example:           "podman container cp [CONTAINER:]SRC_PATH [CONTAINER:]DEST_PATH",
	}
)

var (
	cpOpts entities.ContainerCpOptions
)

func cpFlags(cmd *cobra.Command) {
	flags := cmd.Flags()
	flags.BoolVar(&cpOpts.Extract, "extract", false, "Extract the tar file into the destination directory.")
	flags.BoolVar(&cpOpts.Pause, "pause", true, "Pause the container while copying")
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode},
		Command: cpCommand,
	})
	cpFlags(cpCommand)

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode},
		Command: containerCpCommand,
		Parent:  containerCmd,
	})
	cpFlags(containerCpCommand)
}

func cp(cmd *cobra.Command, args []string) error {
	return registry.ContainerEngine().ContainerCp(registry.GetContext(), args[0], args[1], cpOpts)
}
