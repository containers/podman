package containers

import (
	"github.com/containers/podman/v4/cmd/podman/common"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	cpDescription = `Copy the contents of SRC_PATH to the DEST_PATH.

  You can copy from the container's file system to the local machine or the reverse, from the local filesystem to the container. If "-" is specified for either the SRC_PATH or DEST_PATH, you can also stream a tar archive from STDIN or to STDOUT. The CONTAINER can be a running or stopped container. The SRC_PATH or DEST_PATH can be a file or a directory.
`
	cpCommand = &cobra.Command{
		Use:               "cp [options] [CONTAINER:]SRC_PATH [CONTAINER:]DEST_PATH",
		Short:             "Copy files/folders between a container and the local filesystem",
		Long:              cpDescription,
		Args:              cobra.ExactArgs(2),
		RunE:              cp,
		ValidArgsFunction: common.AutocompleteCpCommand,
		Example:           "podman cp [options] [CONTAINER:]SRC_PATH [CONTAINER:]DEST_PATH",
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
	chown  bool
)

func cpFlags(cmd *cobra.Command) {
	flags := cmd.Flags()
	flags.BoolVar(&cpOpts.OverwriteDirNonDir, "overwrite", false, "Allow to overwrite directories with non-directories and vice versa")
	flags.BoolVarP(&chown, "archive", "a", true, `Chown copied files to the primary uid/gid of the destination container.`)

	// Deprecated flags (both are NOPs): exist for backwards compat
	flags.BoolVar(&cpOpts.Extract, "extract", false, "Deprecated...")
	_ = flags.MarkHidden("extract")
	flags.BoolVar(&cpOpts.Pause, "pause", true, "Deprecated")
	_ = flags.MarkHidden("pause")
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: cpCommand,
	})
	cpFlags(cpCommand)

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: containerCpCommand,
		Parent:  containerCmd,
	})
	cpFlags(containerCpCommand)
}
