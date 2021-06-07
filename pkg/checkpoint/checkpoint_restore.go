package checkpoint

import (
	"context"
	"io/ioutil"
	"os"

	metadata "github.com/checkpoint-restore/checkpointctl/lib"
	"github.com/containers/common/libimage"
	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v3/libpod"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/containers/podman/v3/pkg/errorhandling"
	"github.com/containers/podman/v3/pkg/specgen/generate"
	"github.com/containers/storage/pkg/archive"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// Prefixing the checkpoint/restore related functions with 'cr'

// CRImportCheckpoint it the function which imports the information
// from checkpoint tarball and re-creates the container from that information
func CRImportCheckpoint(ctx context.Context, runtime *libpod.Runtime, restoreOptions entities.RestoreOptions) ([]*libpod.Container, error) {
	// First get the container definition from the
	// tarball to a temporary directory
	archiveFile, err := os.Open(restoreOptions.Import)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open checkpoint archive for import")
	}
	defer errorhandling.CloseQuiet(archiveFile)
	options := &archive.TarOptions{
		// Here we only need the files config.dump and spec.dump
		ExcludePatterns: []string{
			"volumes",
			"ctr.log",
			"artifacts",
			metadata.RootFsDiffTar,
			metadata.DeletedFilesFile,
			metadata.NetworkStatusFile,
			metadata.CheckpointDirectory,
		},
	}
	dir, err := ioutil.TempDir("", "checkpoint")
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := os.RemoveAll(dir); err != nil {
			logrus.Errorf("could not recursively remove %s: %q", dir, err)
		}
	}()
	err = archive.Untar(archiveFile, dir, options)
	if err != nil {
		return nil, errors.Wrapf(err, "Unpacking of checkpoint archive %s failed", restoreOptions.Import)
	}

	// Load spec.dump from temporary directory
	dumpSpec := new(spec.Spec)
	if _, err := metadata.ReadJSONFile(dumpSpec, dir, metadata.SpecDumpFile); err != nil {
		return nil, err
	}

	// Load config.dump from temporary directory
	ctrConfig := new(libpod.ContainerConfig)
	if _, err = metadata.ReadJSONFile(ctrConfig, dir, metadata.ConfigDumpFile); err != nil {
		return nil, err
	}

	// This should not happen as checkpoints with these options are not exported.
	if len(ctrConfig.Dependencies) > 0 {
		return nil, errors.Errorf("Cannot import checkpoints of containers with dependencies")
	}

	// Volumes included in the checkpoint should not exist
	if !restoreOptions.IgnoreVolumes {
		for _, vol := range ctrConfig.NamedVolumes {
			exists, err := runtime.HasVolume(vol.Name)
			if err != nil {
				return nil, err
			}
			if exists {
				return nil, errors.Errorf("volume with name %s already exists. Use --ignore-volumes to not restore content of volumes", vol.Name)
			}
		}
	}

	ctrID := ctrConfig.ID
	newName := false

	// Check if the restored container gets a new name
	if restoreOptions.Name != "" {
		ctrConfig.ID = ""
		ctrConfig.Name = restoreOptions.Name
		newName = true
	}

	if len(restoreOptions.PublishPorts) > 0 {
		ports, _, _, err := generate.ParsePortMapping(restoreOptions.PublishPorts)
		if err != nil {
			return nil, err
		}
		ctrConfig.PortMappings = ports
	}

	pullOptions := &libimage.PullOptions{}
	pullOptions.Writer = os.Stderr
	if _, err := runtime.LibimageRuntime().Pull(ctx, ctrConfig.RootfsImageName, config.PullPolicyMissing, pullOptions); err != nil {
		return nil, err
	}

	// Now create a new container from the just loaded information
	container, err := runtime.RestoreContainer(ctx, dumpSpec, ctrConfig)
	if err != nil {
		return nil, err
	}

	var containers []*libpod.Container
	if container == nil {
		return nil, nil
	}

	containerConfig := container.Config()
	ctrName := ctrConfig.Name
	if containerConfig.Name != ctrName {
		return nil, errors.Errorf("Name of restored container (%s) does not match requested name (%s)", containerConfig.Name, ctrName)
	}

	if !newName {
		// Only check ID for a restore with the same name.
		// Using -n to request a new name for the restored container, will also create a new ID
		if containerConfig.ID != ctrID {
			return nil, errors.Errorf("ID of restored container (%s) does not match requested ID (%s)", containerConfig.ID, ctrID)
		}
	}

	// Check if the ExitCommand points to the correct container ID
	if containerConfig.ExitCommand[len(containerConfig.ExitCommand)-1] != containerConfig.ID {
		return nil, errors.Errorf("'ExitCommandID' uses ID %s instead of container ID %s", containerConfig.ExitCommand[len(containerConfig.ExitCommand)-1], containerConfig.ID)
	}

	containers = append(containers, container)
	return containers, nil
}
