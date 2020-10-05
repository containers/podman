package checkpoint

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/containers/podman/v2/libpod"
	"github.com/containers/podman/v2/libpod/image"
	"github.com/containers/podman/v2/pkg/errorhandling"
	"github.com/containers/podman/v2/pkg/util"
	"github.com/containers/storage/pkg/archive"
	jsoniter "github.com/json-iterator/go"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// Prefixing the checkpoint/restore related functions with 'cr'

// crImportFromJSON imports the JSON files stored in the exported
// checkpoint tarball
func crImportFromJSON(filePath string, v interface{}) error {
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		return errors.Wrap(err, "failed to read container definition for restore")
	}
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	if err = json.Unmarshal(content, v); err != nil {
		return errors.Wrapf(err, "failed to unmarshal container definition %s for restore", filePath)
	}

	return nil
}

// CRImportCheckpoint it the function which imports the information
// from checkpoint tarball and re-creates the container from that information
func CRImportCheckpoint(ctx context.Context, runtime *libpod.Runtime, input string, name string) ([]*libpod.Container, error) {
	// First get the container definition from the
	// tarball to a temporary directory
	archiveFile, err := os.Open(input)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open checkpoint archive for import")
	}
	defer errorhandling.CloseQuiet(archiveFile)
	options := &archive.TarOptions{
		// Here we only need the files config.dump and spec.dump
		ExcludePatterns: []string{
			"checkpoint",
			"artifacts",
			"ctr.log",
			"rootfs-diff.tar",
			"network.status",
			"deleted.files",
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
		return nil, errors.Wrapf(err, "Unpacking of checkpoint archive %s failed", input)
	}

	// Load spec.dump from temporary directory
	dumpSpec := new(spec.Spec)
	if err := crImportFromJSON(filepath.Join(dir, "spec.dump"), dumpSpec); err != nil {
		return nil, err
	}

	// Load config.dump from temporary directory
	config := new(libpod.ContainerConfig)
	if err = crImportFromJSON(filepath.Join(dir, "config.dump"), config); err != nil {
		return nil, err
	}

	// This should not happen as checkpoints with these options are not exported.
	if (len(config.Dependencies) > 0) || (len(config.NamedVolumes) > 0) {
		return nil, errors.Errorf("Cannot import checkpoints of containers with named volumes or dependencies")
	}

	ctrID := config.ID
	newName := false

	// Check if the restored container gets a new name
	if name != "" {
		config.ID = ""
		config.Name = name
		newName = true
	}

	ctrName := config.Name

	// The code to load the images is copied from create.go
	// In create.go this only set if '--quiet' does not exist.
	writer := os.Stderr
	rtc, err := runtime.GetConfig()
	if err != nil {
		return nil, err
	}

	_, err = runtime.ImageRuntime().New(ctx, config.RootfsImageName, rtc.Engine.SignaturePolicyPath, "", writer, nil, image.SigningOptions{}, nil, util.PullImageMissing)
	if err != nil {
		return nil, err
	}

	// Now create a new container from the just loaded information
	container, err := runtime.RestoreContainer(ctx, dumpSpec, config)
	if err != nil {
		return nil, err
	}

	var containers []*libpod.Container
	if container == nil {
		return nil, nil
	}

	containerConfig := container.Config()
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
