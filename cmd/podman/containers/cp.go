package containers

import (
	"github.com/containers/libpod/cmd/podman/registry"
	"github.com/containers/libpod/pkg/cgroups"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	cpDescription = `Command copies the contents of SRC_PATH to the DEST_PATH.

  You can copy from the container's file system to the local machine or the reverse, from the local filesystem to the container. If "-" is specified for either the SRC_PATH or DEST_PATH, you can also stream a tar archive from STDIN or to STDOUT. The CONTAINER can be a running or stopped container.  The SRC_PATH or DEST_PATH can be a file or directory.
`
	cpCommand = &cobra.Command{
		Use:     "cp [flags] SRC_PATH DEST_PATH",
		Short:   "Copy files/folders between a container and the local filesystem",
		Long:    cpDescription,
		Args:    cobra.ExactArgs(2),
		RunE:    cp,
		Example: "podman cp [CONTAINER:]SRC_PATH [CONTAINER:]DEST_PATH",
	}

	containerCpCommand = &cobra.Command{
		Use:     cpCommand.Use,
		Short:   cpCommand.Short,
		Long:    cpCommand.Long,
		Args:    cpCommand.Args,
		RunE:    cpCommand.RunE,
		Example: "podman container cp [CONTAINER:]SRC_PATH [CONTAINER:]DEST_PATH",
	}
)

var (
	cpOpts entities.ContainerCpOptions
)

func cpFlags(flags *pflag.FlagSet) {
	flags.BoolVar(&cpOpts.Extract, "extract", false, "Extract the tar file into the destination directory.")
	flags.BoolVar(&cpOpts.Pause, "pause", copyPause(), "Pause the container while copying")
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode},
		Command: cpCommand,
	})
	flags := cpCommand.Flags()
	cpFlags(flags)

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode},
		Command: containerCpCommand,
		Parent:  containerCmd,
	})
	containerCpFlags := containerCpCommand.Flags()
	cpFlags(containerCpFlags)
}

func cp(cmd *cobra.Command, args []string) error {
	_, err := registry.ContainerEngine().ContainerCp(registry.GetContext(), args[0], args[1], cpOpts)
	return err
}

func copyPause() bool {
	if rootless.IsRootless() {
		cgroupv2, _ := cgroups.IsCgroup2UnifiedMode()
		if !cgroupv2 {
			logrus.Debugf("defaulting to pause==false on rootless cp in cgroupv1 systems")
			return false
		}
	}
	return true
}
