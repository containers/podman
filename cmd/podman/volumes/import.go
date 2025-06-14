package volumes

import (
	"context"
	"fmt"
	"os"

	"github.com/containers/podman/v5/cmd/podman/common"
	"github.com/containers/podman/v5/cmd/podman/parse"
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
	opts := entities.VolumeImportOptions{}

	filepath := args[1]
	if filepath == "-" {
		opts.Input = os.Stdin
	} else {
		if err := parse.ValidateFileName(filepath); err != nil {
			return err
		}

		targetFile, err := os.Open(filepath)
		if err != nil {
			return fmt.Errorf("unable open input file: %w", err)
		}
		defer targetFile.Close()
		opts.Input = targetFile
	}

	containerEngine := registry.ContainerEngine()
	ctx := context.Background()

	return containerEngine.VolumeImport(ctx, args[0], opts)
}
