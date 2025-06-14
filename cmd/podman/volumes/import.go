package volumes

import (
	"context"

	"github.com/containers/podman/v5/cmd/podman/common"
	"github.com/containers/podman/v5/cmd/podman/registry"
	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	importDescription = `Imports contents into a podman volume from specified tarball (.tar, .tar.gz, .tgz, .bzip, .tar.xz, .txz).`
	importCommand     = &cobra.Command{
		Use:               "import VOLUME [SOURCE]",
		Short:             "Import a tarball contents into a podman volume",
		Long:              importDescription,
		RunE:              importVol,
		Args:              cobra.ExactArgs(2),
		ValidArgsFunction: common.AutocompleteVolumes,
		Example: `podman volume import my_vol /home/user/import.tar
  cat ctr.tar | podman volume import my_vol -`,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: importCommand,
		Parent:  volumeCmd,
	})
}

func importVol(cmd *cobra.Command, args []string) error {
	filePath := args[1]
	if filePath == "-" {
		filePath = "/dev/stdin"
	}

	containerEngine := registry.ContainerEngine()
	ctx := context.Background()

	opts := entities.VolumeImportOptions{
		InputPath: filePath,
	}

	if err := containerEngine.VolumeImport(ctx, args[0], opts); err != nil {
		return err
	}
	return nil
}
