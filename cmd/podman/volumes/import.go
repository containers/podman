package volumes

import (
	"errors"
	"fmt"
	"os"

	"github.com/containers/podman/v5/cmd/podman/common"
	"github.com/containers/podman/v5/cmd/podman/parse"
	"github.com/containers/podman/v5/cmd/podman/registry"
	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/containers/podman/v5/pkg/errorhandling"
	"github.com/containers/podman/v5/utils"
	"github.com/spf13/cobra"
)

var (
	importDescription = `Imports contents into a podman volume from specified tarball (.tar, .tar.gz, .tgz, .bzip, .tar.xz, .txz).`
	importCommand     = &cobra.Command{
		Annotations:       map[string]string{registry.EngineMode: registry.ABIMode},
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
	var inspectOpts entities.InspectOptions
	var tarFile *os.File
	containerEngine := registry.ContainerEngine()
	ctx := registry.Context()
	// create a slice of volumes since inspect expects slice as arg
	volumes := []string{args[0]}
	tarPath := args[1]

	if tarPath != "-" {
		err := parse.ValidateFileName(tarPath)
		if err != nil {
			return err
		}

		// open tar file
		tarFile, err = os.Open(tarPath)
		if err != nil {
			return err
		}
	} else {
		tarFile = os.Stdin
	}

	inspectOpts.Type = common.VolumeType
	inspectOpts.Type = common.VolumeType
	volumeData, errs, err := containerEngine.VolumeInspect(ctx, volumes, inspectOpts)
	if err != nil {
		return err
	}
	if len(errs) > 0 {
		return errorhandling.JoinErrors(errs)
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
	// dont care if volume is mounted or not we are gonna import everything to mountPoint
	return utils.UntarToFileSystem(mountPoint, tarFile, nil)
}
