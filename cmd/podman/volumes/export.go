package volumes

import (
	"context"
	"fmt"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/podman/v4/cmd/podman/common"
	"github.com/containers/podman/v4/cmd/podman/inspect"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/utils"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	volumeExportDescription = `
podman volume export

Allow content of volume to be exported into external tar.`
	exportCommand = &cobra.Command{
		Annotations:       map[string]string{registry.EngineMode: registry.ABIMode},
		Use:               "export [options] VOLUME",
		Short:             "Export volumes",
		Args:              cobra.ExactArgs(1),
		Long:              volumeExportDescription,
		RunE:              export,
		ValidArgsFunction: common.AutocompleteVolumes,
	}
)

var (
	// Temporary struct to hold cli values.
	cliExportOpts = struct {
		Output string
	}{}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: exportCommand,
		Parent:  volumeCmd,
	})
	flags := exportCommand.Flags()

	outputFlagName := "output"
	flags.StringVarP(&cliExportOpts.Output, outputFlagName, "o", "/dev/stdout", "Write to a specified file (default: stdout, which must be redirected)")
	_ = exportCommand.RegisterFlagCompletionFunc(outputFlagName, completion.AutocompleteDefault)
}

func export(cmd *cobra.Command, args []string) error {
	var inspectOpts entities.InspectOptions
	containerEngine := registry.ContainerEngine()
	ctx := context.Background()

	if cliExportOpts.Output == "" {
		return errors.New("expects output path, use --output=[path]")
	}
	inspectOpts.Type = inspect.VolumeType
	volumeData, _, err := containerEngine.VolumeInspect(ctx, args, inspectOpts)
	if err != nil {
		return err
	}
	if len(volumeData) < 1 {
		return errors.New("no volume data found")
	}
	mountPoint := volumeData[0].VolumeConfigResponse.Mountpoint
	driver := volumeData[0].VolumeConfigResponse.Driver
	volumeOptions := volumeData[0].VolumeConfigResponse.Options
	volumeMountStatus, err := containerEngine.VolumeMounted(ctx, args[0])
	if err != nil {
		return err
	}
	if mountPoint == "" {
		return errors.New("volume is not mounted anywhere on host")
	}
	// Check if volume is using external plugin and export only if volume is mounted
	if driver != "" && driver != "local" {
		if !volumeMountStatus.Value {
			return fmt.Errorf("volume is using a driver %s and volume is not mounted on %s", driver, mountPoint)
		}
	}
	// Check if volume is using `local` driver and has mount options type other than tmpfs
	if driver == "local" {
		if mountOptionType, ok := volumeOptions["type"]; ok {
			if mountOptionType != "tmpfs" && !volumeMountStatus.Value {
				return fmt.Errorf("volume is using a driver %s and volume is not mounted on %s", driver, mountPoint)
			}
		}
	}
	logrus.Debugf("Exporting volume data from %s to %s", mountPoint, cliExportOpts.Output)
	err = utils.CreateTarFromSrc(mountPoint, cliExportOpts.Output)
	return err
}
