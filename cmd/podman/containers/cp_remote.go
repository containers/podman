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
	"github.com/spf13/cobra"
)

func cp(cmd *cobra.Command, args []string) error {
	// Parse user input.
	sourceContainerStr, sourcePath, destContainerStr, destPath, err := copy.ParseSourceAndDestination(args[0], args[1])
	if err != nil {
		return err
	}

	if len(sourceContainerStr) > 0 && len(destContainerStr) > 0 {
		return errors.New("copying between containers is not supported with podman-remote")
	} else if len(sourceContainerStr) > 0 {
		return copyFromContainerRemote(sourceContainerStr, sourcePath, destPath)
	}

	return copyToContainerRemote(destContainerStr, destPath, sourcePath)
}

// copyFromContainerRemote copies from the containerPath on the container to hostPath.
func copyFromContainerRemote(container string, containerPath string, hostPath string) error {
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

	// Extract tar to destination
	if err := extractTar(reader, hostPath, containerInfo.IsDir); err != nil {
		return err
	}

	return copyErr
}

// copyToContainerRemote copies from hostPath to the containerPath on the container.
func copyToContainerRemote(container string, containerPath string, hostPath string) error {
	isStdin := hostPath == "-"
	if isStdin {
		hostPath = os.Stdin.Name()
	}

	// Get info about the host path
	hostInfo, err := os.Stat(hostPath)
	if err != nil {
		return fmt.Errorf("%q could not be found on the host: %w", hostPath, err)
	}

	// Get info about the container destination path
	containerInfo, err := registry.ContainerEngine().ContainerStat(registry.GetContext(), container, containerPath)
	var containerExists bool
	var containerIsDir bool
	var containerResolvedToParentDir bool
	var targetPath string
	var containerBaseName string

	if err != nil {
		// Container path doesn't exist
		// If path has trailing /, it must be a directory (error if it doesn't exist)
		if strings.HasSuffix(containerPath, "/") {
			return fmt.Errorf("%q could not be found on container %s: %w", containerPath, container, err)
		}
		containerExists = false

		// If we're copying contents only (source ends with /.), use the dest path directly
		// The server will create it as a directory
		if strings.HasSuffix(hostPath, "/.") {
			targetPath = containerPath
			containerResolvedToParentDir = false
		} else {
			// Otherwise, use parent directory and rename
			containerResolvedToParentDir = true
			targetPath = filepath.Dir(containerPath)
			containerBaseName = filepath.Base(containerPath)
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
	if hostInfo.IsDir() && containerExists && !containerIsDir {
		return errors.New("destination must be a directory when copying a directory")
	}

	reader, writer := io.Pipe()

	// Create tar archive from host path in a goroutine
	var tarErr error
	go func() {
		defer writer.Close()
		if isStdin {
			_, tarErr = io.Copy(writer, os.Stdin)
		} else {
			tarErr = createTar(hostPath, writer)
		}
	}()

	// Copy the archive to the container
	defer reader.Close()

	copyOptions := entities.CopyOptions{
		Chown: chown,
	}

	// If we're copying to a non-existent path or file-to-file, use Rename
	// But NOT when copying contents only (hostPath ends with /.)
	if ((!hostInfo.IsDir() && !containerIsDir) || containerResolvedToParentDir) && !strings.HasSuffix(hostPath, "/.") {
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
func extractTar(reader io.Reader, destPath string, isDir bool) error {
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

		var target string
		// If dest doesn't exist and we're extracting a single file, use dest as the filename
		if !destExists && !isDir && header.Typeflag == tar.TypeReg {
			target = destPath
		} else if destIsDir {
			// Dest is a directory, extract into it
			target = filepath.Join(destPath, header.Name)
		} else {
			// Dest exists but isn't a directory, or we're extracting a directory
			target = filepath.Join(destPath, header.Name)
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
