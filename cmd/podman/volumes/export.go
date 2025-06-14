package volumes

import (
	"context"
	"errors"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/podman/v5/cmd/podman/common"
	"github.com/containers/podman/v5/cmd/podman/registry"
	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	volumeExportDescription = `
podman volume export

Allow content of volume to be exported into external tar.`
	exportCommand = &cobra.Command{
		Use:               "export [options] VOLUME",
		Short:             "Export volumes",
		Args:              cobra.ExactArgs(1),
		Long:              volumeExportDescription,
		RunE:              export,
		ValidArgsFunction: common.AutocompleteVolumes,
	}
)

var (
	cliExportOpts entities.VolumeExportOptions
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: exportCommand,
		Parent:  volumeCmd,
	})
	flags := exportCommand.Flags()

	outputFlagName := "output"
	flags.StringVarP(&cliExportOpts.OutputPath, outputFlagName, "o", "/dev/stdout", "Write to a specified file (default: stdout, which must be redirected)")
	_ = exportCommand.RegisterFlagCompletionFunc(outputFlagName, completion.AutocompleteDefault)
}

func export(cmd *cobra.Command, args []string) error {
	containerEngine := registry.ContainerEngine()
	ctx := context.Background()

	if cliExportOpts.OutputPath == "" {
		return errors.New("expects output path, use --output=[path]")
	}

	if err := containerEngine.VolumeExport(ctx, args[0], cliExportOpts); err != nil {
		return err
	}
	return nil
}
