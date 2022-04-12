package crutils

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	metadata "github.com/checkpoint-restore/checkpointctl/lib"
	"github.com/checkpoint-restore/go-criu/v5/stats"
	"github.com/containers/storage/pkg/archive"
	"github.com/opencontainers/selinux/go-selinux/label"
	"github.com/pkg/errors"
)

// This file mainly exist to make the checkpoint/restore functions
// available for other users. One possible candidate would be CRI-O.

// CRImportCheckpointWithoutConfig imports the checkpoint archive (input)
// into the directory destination without "config.dump" and "spec.dump"
func CRImportCheckpointWithoutConfig(destination, input string) error {
	archiveFile, err := os.Open(input)
	if err != nil {
		return errors.Wrapf(err, "Failed to open checkpoint archive %s for import", input)
	}

	defer archiveFile.Close()
	options := &archive.TarOptions{
		ExcludePatterns: []string{
			// Import everything else besides the container config
			metadata.ConfigDumpFile,
			metadata.SpecDumpFile,
		},
	}
	if err = archive.Untar(archiveFile, destination, options); err != nil {
		return errors.Wrapf(err, "Unpacking of checkpoint archive %s failed", input)
	}

	return nil
}

// CRImportCheckpointConfigOnly only imports the checkpoint configuration
// from the checkpoint archive (input) into the directory destination.
// Only the files "config.dump" and "spec.dump" are extracted.
func CRImportCheckpointConfigOnly(destination, input string) error {
	archiveFile, err := os.Open(input)
	if err != nil {
		return errors.Wrapf(err, "Failed to open checkpoint archive %s for import", input)
	}

	defer archiveFile.Close()
	options := &archive.TarOptions{
		// Here we only need the files config.dump and spec.dump
		ExcludePatterns: []string{
			"ctr.log",
			"artifacts",
			stats.StatsDump,
			metadata.RootFsDiffTar,
			metadata.DeletedFilesFile,
			metadata.NetworkStatusFile,
			metadata.CheckpointDirectory,
			metadata.CheckpointVolumesDirectory,
		},
	}
	if err = archive.Untar(archiveFile, destination, options); err != nil {
		return errors.Wrapf(err, "Unpacking of checkpoint archive %s failed", input)
	}

	return nil
}

// CRRemoveDeletedFiles loads the list of deleted files and if
// it exists deletes all files listed.
func CRRemoveDeletedFiles(id, baseDirectory, containerRootDirectory string) error {
	deletedFiles, _, err := metadata.ReadContainerCheckpointDeletedFiles(baseDirectory)
	if os.IsNotExist(errors.Unwrap(errors.Unwrap(err))) {
		// No files to delete. Just return
		return nil
	}

	if err != nil {
		return errors.Wrapf(err, "failed to read deleted files file")
	}

	for _, deleteFile := range deletedFiles {
		// Using RemoveAll as deletedFiles, which is generated from 'podman diff'
		// lists completely deleted directories as a single entry: 'D /root'.
		if err := os.RemoveAll(filepath.Join(containerRootDirectory, deleteFile)); err != nil {
			return errors.Wrapf(err, "failed to delete files from container %s during restore", id)
		}
	}

	return nil
}

// CRApplyRootFsDiffTar applies the tar archive found in baseDirectory with the
// root file system changes on top of containerRootDirectory
func CRApplyRootFsDiffTar(baseDirectory, containerRootDirectory string) error {
	rootfsDiffPath := filepath.Join(baseDirectory, metadata.RootFsDiffTar)
	// Only do this if a rootfs-diff.tar actually exists
	rootfsDiffFile, err := os.Open(rootfsDiffPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return errors.Wrap(err, "failed to open root file-system diff file")
	}
	defer rootfsDiffFile.Close()

	if err := archive.Untar(rootfsDiffFile, containerRootDirectory, nil); err != nil {
		return errors.Wrapf(err, "failed to apply root file-system diff file %s", rootfsDiffPath)
	}

	return nil
}

// CRCreateRootFsDiffTar goes through the 'changes' and can create two files:
// * metadata.RootFsDiffTar will contain all new and changed files
// * metadata.DeletedFilesFile will contain a list of deleted files
// With these two files it is possible to restore the container file system to the same
// state it was during checkpointing.
// Changes to directories (owner, mode) are not handled.
func CRCreateRootFsDiffTar(changes *[]archive.Change, mountPoint, destination string) (includeFiles []string, err error) {
	if len(*changes) == 0 {
		return includeFiles, nil
	}

	var rootfsIncludeFiles []string
	var deletedFiles []string

	rootfsDiffPath := filepath.Join(destination, metadata.RootFsDiffTar)

	for _, file := range *changes {
		if file.Kind == archive.ChangeAdd {
			rootfsIncludeFiles = append(rootfsIncludeFiles, file.Path)
			continue
		}
		if file.Kind == archive.ChangeDelete {
			deletedFiles = append(deletedFiles, file.Path)
			continue
		}
		fileName, err := os.Stat(file.Path)
		if err != nil {
			continue
		}
		if !fileName.IsDir() && file.Kind == archive.ChangeModify {
			rootfsIncludeFiles = append(rootfsIncludeFiles, file.Path)
			continue
		}
	}

	if len(rootfsIncludeFiles) > 0 {
		rootfsTar, err := archive.TarWithOptions(mountPoint, &archive.TarOptions{
			Compression:      archive.Uncompressed,
			IncludeSourceDir: true,
			IncludeFiles:     rootfsIncludeFiles,
		})
		if err != nil {
			return includeFiles, errors.Wrapf(err, "error exporting root file-system diff to %q", rootfsDiffPath)
		}
		rootfsDiffFile, err := os.Create(rootfsDiffPath)
		if err != nil {
			return includeFiles, errors.Wrapf(err, "error creating root file-system diff file %q", rootfsDiffPath)
		}
		defer rootfsDiffFile.Close()
		if _, err = io.Copy(rootfsDiffFile, rootfsTar); err != nil {
			return includeFiles, err
		}

		includeFiles = append(includeFiles, metadata.RootFsDiffTar)
	}

	if len(deletedFiles) == 0 {
		return includeFiles, nil
	}

	if _, err := metadata.WriteJSONFile(deletedFiles, destination, metadata.DeletedFilesFile); err != nil {
		return includeFiles, nil
	}

	includeFiles = append(includeFiles, metadata.DeletedFilesFile)

	return includeFiles, nil
}

// CRCreateFileWithLabel creates an empty file and sets the corresponding ('fileLabel')
// SELinux label on the file.
// This is necessary for CRIU log files because CRIU infects the processes in
// the container with a 'parasite' and this will also try to write to the log files
// from the context of the container processes.
func CRCreateFileWithLabel(directory, fileName, fileLabel string) error {
	logFileName := filepath.Join(directory, fileName)

	logFile, err := os.OpenFile(logFileName, os.O_CREATE, 0o600)
	if err != nil {
		return errors.Wrapf(err, "failed to create file %q", logFileName)
	}
	defer logFile.Close()
	if err = label.SetFileLabel(logFileName, fileLabel); err != nil {
		return errors.Wrapf(err, "failed to label file %q", logFileName)
	}

	return nil
}

// CRRuntimeSupportsCheckpointRestore tests if the given runtime at 'runtimePath'
// supports checkpointing. The checkpoint restore interface has no definition
// but crun implements all commands just as runc does. Whathh runc does it the
// official definition of the checkpoint/restore interface.
func CRRuntimeSupportsCheckpointRestore(runtimePath string) bool {
	// Check if the runtime implements checkpointing. Currently only
	// runc's and crun's checkpoint/restore implementation is supported.
	cmd := exec.Command(runtimePath, "checkpoint", "--help")
	if err := cmd.Start(); err != nil {
		return false
	}
	if err := cmd.Wait(); err == nil {
		return true
	}
	return false
}

// CRRuntimeSupportsCheckpointRestore tests if the runtime at 'runtimePath'
// supports restoring into existing Pods. The runtime needs to support
// the CRIU option --lsm-mount-context and the existence of this is checked
// by this function. In addition it is necessary to at least have CRIU 3.16.
func CRRuntimeSupportsPodCheckpointRestore(runtimePath string) bool {
	cmd := exec.Command(runtimePath, "restore", "--lsm-mount-context")
	out, _ := cmd.CombinedOutput()
	return bytes.Contains(out, []byte("flag needs an argument"))
}

// CRGetRuntimeFromArchive extracts the checkpoint metadata from the
// given checkpoint archive and returns the runtime used to create
// the given checkpoint archive.
func CRGetRuntimeFromArchive(input string) (*string, error) {
	dir, err := ioutil.TempDir("", "checkpoint")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)

	if err := CRImportCheckpointConfigOnly(dir, input); err != nil {
		return nil, err
	}

	// Load config.dump from temporary directory
	ctrConfig := new(metadata.ContainerConfig)
	if _, err = metadata.ReadJSONFile(ctrConfig, dir, metadata.ConfigDumpFile); err != nil {
		return nil, err
	}

	return &ctrConfig.OCIRuntime, nil
}
