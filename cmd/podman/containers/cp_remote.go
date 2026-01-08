//go:build remote

package containers

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/pkg/copy"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/storage/pkg/archive"
	"github.com/spf13/cobra"
)

func cp(cmd *cobra.Command, args []string) error {
	// Parse user input.
	sourceContainerStr, sourcePath, destContainerStr, destPath, err := copy.ParseSourceAndDestination(args[0], args[1])
	if err != nil {
		return err
	}

	if len(sourceContainerStr) > 0 && len(destContainerStr) > 0 {
		return copyBetweenContainersRemote(sourceContainerStr, sourcePath, destContainerStr, destPath)
	} else if len(sourceContainerStr) > 0 {
		return copyFromContainerRemote(sourceContainerStr, sourcePath, destPath)
	}

	return copyToContainerRemote(destContainerStr, destPath, sourcePath)
}

// copyFromContainerRemote copies from the containerPath on the container to hostPath.
func copyFromContainerRemote(container string, containerPath string, hostPath string) error {
	// Validate: /dev/stdout is not a valid destination
	// Only "-" gets special treatment as stdout
	if hostPath == "/dev/stdout" {
		return fmt.Errorf("invalid destination: %q must be a directory or a regular file", hostPath)
	}

	isStdout := hostPath == "-"
	if isStdout {
		hostPath = os.Stdout.Name()
	}

	// Get the file info from the container to know what we're copying
	containerInfo, err := registry.ContainerEngine().ContainerStat(registry.GetContext(), container, containerPath)
	if err != nil {
		return fmt.Errorf("%q could not be found on container %s: %w", containerPath, container, err)
	}

	reader, writer := io.Pipe()

	// Start copying from the container in a goroutine
	var copyErr error
	go func() {
		defer writer.Close()
		copyFunc, err := registry.ContainerEngine().ContainerCopyToArchive(registry.GetContext(), container, containerInfo.LinkTarget, writer)
		if err != nil {
			copyErr = err
			return
		}
		copyErr = copyFunc()
	}()

	// Extract the tar archive to the host
	defer reader.Close()

	if isStdout {
		_, err := io.Copy(os.Stdout, reader)
		return err
	}

	// Validate: when copying a directory, destination must be a directory or non-existent
	if containerInfo.IsDir {
		if hostInfo, err := os.Stat(hostPath); err == nil {
			// Destination exists, check if it's a directory
			if !hostInfo.IsDir() {
				return errors.New("destination must be a directory when copying a directory")
			}
		}
	}

	// Extract tar to destination
	// When copying a directory to a non-existent destination, we need to strip
	// the source directory name from tar entries. For example, when copying
	// /srv to /newdir, the tar contains "srv/subdir/file" but we want to extract
	// to "/newdir/subdir/file" not "/newdir/srv/subdir/file".
	stripComponents := 0
	if containerInfo.IsDir {
		// Check if destination exists
		if _, err := os.Stat(hostPath); os.IsNotExist(err) {
			// Destination doesn't exist, strip the source directory name
			// unless we're copying contents only (path ends with /.)
			if !strings.HasSuffix(containerPath, "/.") {
				stripComponents = 1
			}
		}
	}

	if err := extractTar(reader, hostPath, containerInfo.IsDir, stripComponents, cpOpts.OverwriteDirNonDir); err != nil {
		return err
	}

	return copyErr
}

// copyToContainerRemote copies from hostPath to the containerPath on the container.
func copyToContainerRemote(container string, containerPath string, hostPath string) error {
	isStdin := hostPath == "-" || hostPath == "/dev/stdin" || hostPath == os.Stdin.Name()
	var stdinFile string
	if isStdin {
		// Copy from stdin to a temporary file to validate it's a tar archive
		// This provides proper client-side error reporting
		tmpFile, err := os.CreateTemp("", "podman-cp-")
		if err != nil {
			return err
		}
		defer os.Remove(tmpFile.Name())

		_, err = io.Copy(tmpFile, os.Stdin)
		if err != nil {
			tmpFile.Close()
			return err
		}
		if err = tmpFile.Close(); err != nil {
			return err
		}

		if !archive.IsArchivePath(tmpFile.Name()) {
			return errors.New("source must be a (compressed) tar archive when copying from stdin")
		}

		stdinFile = tmpFile.Name()
		hostPath = stdinFile
	}

	// Get info about the host path (skip stat for stdin)
	var hostInfo os.FileInfo
	var err error
	if !isStdin {
		hostInfo, err = os.Stat(hostPath)
		if err != nil {
			return fmt.Errorf("%q could not be found on the host: %w", hostPath, err)
		}
	}

	// Get info about the container destination path
	containerInfo, err := registry.ContainerEngine().ContainerStat(registry.GetContext(), container, containerPath)
	var containerExists bool
	var containerIsDir bool
	var containerResolvedToParentDir bool
	var targetPath string
	var containerBaseName string

	if err != nil {
		// Container path doesn't exist (or is a broken symlink)
		// If path has trailing /, it must be a directory (error if it doesn't exist)
		if strings.HasSuffix(containerPath, "/") {
			return fmt.Errorf("%q could not be found on container %s: %w", containerPath, container, err)
		}

		// If containerInfo is not nil, it's a symlink even if target doesn't exist
		if containerInfo != nil && containerInfo.LinkTarget != "" {
			// Broken symlink - treat like a file and use the symlink target
			containerExists = true
			containerIsDir = false
			targetPath = filepath.Dir(containerInfo.LinkTarget)
			containerBaseName = filepath.Base(containerInfo.LinkTarget)
		} else {
			// Path truly doesn't exist
			containerExists = false

			// When copying from stdin or copying contents only (source ends with /.),
			// use the dest path directly - the server will create it as a directory
			if isStdin || strings.HasSuffix(hostPath, "/.") {
				targetPath = containerPath
				containerResolvedToParentDir = false
			} else {
				// Otherwise, use parent directory and rename
				containerResolvedToParentDir = true
				targetPath = filepath.Dir(containerPath)
				containerBaseName = filepath.Base(containerPath)
			}
		}
	} else {
		containerExists = true
		containerIsDir = containerInfo.IsDir
		if containerIsDir {
			// Destination is a directory - extract into it
			targetPath = containerPath
		} else {
			// Destination is a file - use parent directory
			targetPath = filepath.Dir(containerInfo.LinkTarget)
			containerBaseName = filepath.Base(containerInfo.LinkTarget)
		}
	}

	// Validate: can't copy directory to a file
	if !isStdin && hostInfo.IsDir() && containerExists && !containerIsDir {
		return errors.New("destination must be a directory when copying a directory")
	}

	// When copying from stdin, destination must exist and be a directory
	if isStdin {
		if !containerExists || !containerIsDir {
			return errors.New("destination must be a directory when copying from stdin")
		}
	}

	reader, writer := io.Pipe()

	// Create tar archive from host path in a goroutine
	var tarErr error
	go func() {
		defer writer.Close()
		if isStdin {
			// Read from the temp file we created
			f, err := os.Open(stdinFile)
			if err != nil {
				tarErr = err
				return
			}
			defer f.Close()
			_, tarErr = io.Copy(writer, f)
		} else {
			tarErr = createTar(hostPath, writer)
		}
	}()

	// Copy the archive to the container
	defer reader.Close()

	copyOptions := entities.CopyOptions{
		Chown:                chown,
		NoOverwriteDirNonDir: !cpOpts.OverwriteDirNonDir,
	}

	// If we're copying to a non-existent path or file-to-file, use Rename
	// But NOT when copying from stdin or when copying contents only (hostPath ends with /.)
	if !isStdin && ((!hostInfo.IsDir() && !containerIsDir) || containerResolvedToParentDir) && !strings.HasSuffix(hostPath, "/.") {
		copyOptions.Rename = map[string]string{filepath.Base(hostPath): containerBaseName}
	}

	copyFunc, err := registry.ContainerEngine().ContainerCopyFromArchive(registry.GetContext(), container, targetPath, reader, copyOptions)
	if err != nil {
		return err
	}

	if err := copyFunc(); err != nil {
		return err
	}

	return tarErr
}

// copyBetweenContainersRemote copies from source container to destination container.
func copyBetweenContainersRemote(sourceContainer string, sourcePath string, destContainer string, destPath string) error {
	// Get the file info from the source container
	sourceInfo, err := registry.ContainerEngine().ContainerStat(registry.GetContext(), sourceContainer, sourcePath)
	if err != nil {
		return fmt.Errorf("%q could not be found on container %s: %w", sourcePath, sourceContainer, err)
	}

	// Get info about the destination container path
	destInfo, err := registry.ContainerEngine().ContainerStat(registry.GetContext(), destContainer, destPath)
	var destExists bool
	var destIsDir bool
	var destResolvedToParentDir bool
	var targetPath string
	var destBaseName string

	if err != nil {
		// Destination path doesn't exist (or is a broken symlink)
		// If path has trailing /, it must be a directory (error if it doesn't exist)
		if strings.HasSuffix(destPath, "/") {
			return fmt.Errorf("%q could not be found on container %s: %w", destPath, destContainer, err)
		}

		// If destInfo is not nil, it's a symlink even if target doesn't exist
		if destInfo != nil && destInfo.LinkTarget != "" {
			// Broken symlink - treat like a file and use the symlink target
			destExists = true
			destIsDir = false
			targetPath = filepath.Dir(destInfo.LinkTarget)
			destBaseName = filepath.Base(destInfo.LinkTarget)
		} else {
			// Path truly doesn't exist
			destExists = false

			// If we're copying contents only (source ends with /.), use the dest path directly
			if strings.HasSuffix(sourcePath, "/.") {
				targetPath = destPath
				destResolvedToParentDir = false
			} else {
				// Otherwise, use parent directory and rename
				destResolvedToParentDir = true
				targetPath = filepath.Dir(destPath)
				destBaseName = filepath.Base(destPath)
			}
		}
	} else {
		destExists = true
		destIsDir = destInfo.IsDir
		if destIsDir {
			// Destination is a directory - extract into it
			targetPath = destPath
		} else {
			// Destination is a file - use parent directory
			targetPath = filepath.Dir(destInfo.LinkTarget)
			destBaseName = filepath.Base(destInfo.LinkTarget)
		}
	}

	// Validate: can't copy directory to a file
	if sourceInfo.IsDir && destExists && !destIsDir {
		return errors.New("destination must be a directory when copying a directory")
	}

	reader, writer := io.Pipe()

	// Copy from source container in a goroutine
	var copyFromErr error
	go func() {
		defer writer.Close()
		copyFunc, err := registry.ContainerEngine().ContainerCopyToArchive(registry.GetContext(), sourceContainer, sourceInfo.LinkTarget, writer)
		if err != nil {
			copyFromErr = err
			return
		}
		copyFromErr = copyFunc()
	}()

	// Copy to destination container
	defer reader.Close()

	copyOptions := entities.CopyOptions{
		Chown:                chown,
		NoOverwriteDirNonDir: !cpOpts.OverwriteDirNonDir,
	}

	// If we're copying to a non-existent path or file-to-file, use Rename
	// But NOT when copying contents only (sourcePath ends with /.)
	if ((!sourceInfo.IsDir && !destIsDir) || destResolvedToParentDir) && !strings.HasSuffix(sourcePath, "/.") {
		copyOptions.Rename = map[string]string{filepath.Base(sourceInfo.LinkTarget): destBaseName}
	}

	copyFunc, err := registry.ContainerEngine().ContainerCopyFromArchive(registry.GetContext(), destContainer, targetPath, reader, copyOptions)
	if err != nil {
		return err
	}

	if err := copyFunc(); err != nil {
		return err
	}

	return copyFromErr
}

// createTar creates a tar archive from the specified path
func createTar(sourcePath string, writer io.Writer) error {
	tw := tar.NewWriter(writer)
	defer tw.Close()

	// Check if we should copy contents only (path ends with /.)
	// vs copying the directory itself
	// Must check BEFORE cleaning, as Clean will remove the trailing /.
	keepDirName := !strings.HasSuffix(sourcePath, "/.")
	cleanPath := filepath.Clean(sourcePath)
	baseDir := filepath.Dir(cleanPath)

	return filepath.Walk(cleanPath, func(file string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		header, err := tar.FileInfoHeader(fi, file)
		if err != nil {
			return err
		}

		// Update the name to be relative
		var relPath string
		if keepDirName {
			// Include the directory name in the archive
			relPath, err = filepath.Rel(baseDir, file)
		} else {
			// Copy contents only, exclude the directory name
			relPath, err = filepath.Rel(cleanPath, file)
			if relPath == "." {
				// Skip the directory itself when copying contents
				return nil
			}
		}
		if err != nil {
			return err
		}
		header.Name = relPath

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if !fi.IsDir() {
			f, err := os.Open(file)
			if err != nil {
				return err
			}
			defer f.Close()

			if _, err := io.Copy(tw, f); err != nil {
				return err
			}
		}

		return nil
	})
}

// extractTar extracts a tar archive to the specified destination
func extractTar(reader io.Reader, destPath string, isDir bool, stripComponents int, overwrite bool) error {
	tr := tar.NewReader(reader)

	// Check if destination exists
	destInfo, destErr := os.Stat(destPath)
	destExists := destErr == nil
	destIsDir := destExists && destInfo.IsDir()

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Strip leading path components if requested
		name := header.Name
		if stripComponents > 0 {
			parts := strings.Split(filepath.Clean(name), string(filepath.Separator))
			if len(parts) > stripComponents {
				name = filepath.Join(parts[stripComponents:]...)
			} else {
				// Skip entries that would be completely stripped
				continue
			}
		}

		var target string
		// If dest doesn't exist and we're extracting a single file, use dest as the filename
		if !destExists && !isDir && header.Typeflag == tar.TypeReg {
			target = destPath
		} else if destIsDir {
			// Dest is a directory, extract into it
			target = filepath.Join(destPath, name)
		} else {
			// Dest exists but isn't a directory, or we're extracting a directory
			target = filepath.Join(destPath, name)
		}

		// Check if target exists and handle overwrite
		if targetInfo, err := os.Lstat(target); err == nil {
			targetIsDir := targetInfo.IsDir()
			// If types don't match (file vs directory)
			if (header.Typeflag == tar.TypeDir && !targetIsDir) || (header.Typeflag == tar.TypeReg && targetIsDir) {
				if !overwrite {
					return fmt.Errorf("error creating %q: file exists", filepath.Join("/", name))
				}
				// Remove the existing path to allow overwrite
				if err := os.RemoveAll(target); err != nil {
					return err
				}
			}
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			// Ensure parent directory exists
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}

			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return err
			}

			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()
		}
	}

	return nil
}
