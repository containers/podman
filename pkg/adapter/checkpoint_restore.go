// +build !remoteclient

package adapter

import (
	"context"
	"github.com/containers/libpod/libpod"
	"github.com/containers/storage/pkg/archive"
	jsoniter "github.com/json-iterator/go"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"io/ioutil"
	"os"
	"path/filepath"
)

// Prefixing the checkpoint/restore related functions with 'cr'

// crImportFromJSON imports the JSON files stored in the exported
// checkpoint tarball
func crImportFromJSON(filePath string, v interface{}) error {
	jsonFile, err := os.Open(filePath)
	if err != nil {
		return errors.Wrapf(err, "Failed to open container definition %s for restore", filePath)
	}
	defer jsonFile.Close()

	content, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		return errors.Wrapf(err, "Failed to read container definition %s for restore", filePath)
	}
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	if err = json.Unmarshal([]byte(content), v); err != nil {
		return errors.Wrapf(err, "Failed to unmarshal container definition %s for restore", filePath)
	}

	return nil
}

// crImportCheckpoint it the function which imports the information
// from checkpoint tarball and re-creates the container from that information
func crImportCheckpoint(ctx context.Context, runtime *libpod.Runtime, input string) ([]*libpod.Container, error) {
	// First get the container definition from the
	// tarball to a temporary directory
	archiveFile, err := os.Open(input)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to open checkpoint archive %s for import", input)
	}
	defer archiveFile.Close()
	options := &archive.TarOptions{
		// Here we only need the files config.dump and spec.dump
		ExcludePatterns: []string{
			"checkpoint",
			"artifacts",
			"ctr.log",
			"network.status",
		},
	}
	dir, err := ioutil.TempDir("", "checkpoint")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	err = archive.Untar(archiveFile, dir, options)
	if err != nil {
		return nil, errors.Wrapf(err, "Unpacking of checkpoint archive %s failed", input)
	}

	// Load spec.dump from temporary directory
	spec := new(spec.Spec)
	if err := crImportFromJSON(filepath.Join(dir, "spec.dump"), spec); err != nil {
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

	// Now create a new container from the just loaded information
	container, err := runtime.RestoreContainer(ctx, spec, config)
	if err != nil {
		return nil, err
	}

	var containers []*libpod.Container
	if container == nil {
		return nil, nil
	}

	containers = append(containers, container)
	return containers, nil
}
