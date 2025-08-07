//go:build remote && (amd64 || arm64)

package local_utils

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/containers/podman/v5/pkg/bindings"
	"github.com/containers/podman/v5/pkg/machine/define"
	"github.com/containers/podman/v5/pkg/machine/env"
	"github.com/containers/podman/v5/pkg/machine/provider"
	"github.com/containers/podman/v5/pkg/machine/vmconfigs"
	"github.com/containers/podman/v5/pkg/specgen"
	"github.com/containers/storage/pkg/fileutils"
	"github.com/sirupsen/logrus"
)

// getMachineMounts retrieves the mounts of a machine based on the connection URI and parsed URL.
// It returns a slice of mounts or an error if the machine cannot be found or is not running.
func getMachineMounts(connectionURI string, parsedConnection *url.URL, machineProvider vmconfigs.VMProvider) ([]*vmconfigs.Mount, error) {
	dirs, err := env.GetMachineDirs(machineProvider.VMType())
	if err != nil {
		return nil, err
	}

	machineList, err := vmconfigs.LoadMachinesInDir(dirs)
	if err != nil {
		return nil, fmt.Errorf("listing machines: %w", err)
	}

	// Now we know that the connection points to a machine and we
	// can find the machine by looking for the one with the
	// matching port.
	connectionPort, err := strconv.Atoi(parsedConnection.Port())
	if err != nil {
		return nil, fmt.Errorf("parsing connection port: %w", err)
	}
	for _, mc := range machineList {
		if connectionPort != mc.SSH.Port {
			continue
		}

		state, err := machineProvider.State(mc, false)
		if err != nil {
			return nil, err
		}

		if state != define.Running {
			return nil, fmt.Errorf("machine %s is not running but in state %s", mc.Name, state)
		}
		return mc.Mounts, nil
	}
	return nil, fmt.Errorf("could not find a matching machine for connection %q", connectionURI)
}

// isPathAvailableOnMachine checks if a local path is available on the machine through mounted directories.
// If the path is available, it returns a LocalAPIMap with the corresponding remote path.
func isPathAvailableOnMachine(mounts []*vmconfigs.Mount, vmType define.VMType, path string) (*LocalAPIMap, bool) {
	pathABS, err := filepath.Abs(path)
	if err != nil {
		logrus.Debugf("Failed to get absolute path for %s: %v", path, err)
		return nil, false
	}

	converted_path, err := specgen.ConvertWinMountPath(pathABS)
	if err != nil {
		logrus.Debugf("Failed to convert Windows mount path: %v", err)
		return nil, false
	}

	// WSLVirt is a special case where there is no real concept of doing a mount in WSL,
	// WSL by default mounts the drives to /mnt/c, /mnt/d, etc...
	if vmType == define.WSLVirt {
		return &LocalAPIMap{
			ClientPath: pathABS,
			RemotePath: converted_path,
		}, true
	}

	for _, mount := range mounts {
		mountSource := filepath.Clean(mount.Source)
		if strings.HasPrefix(converted_path, mountSource) {
			// Ensure we're matching directory boundaries, not just prefixes
			// e.g., /home/user should not match /home/username
			if len(converted_path) > len(mountSource) && converted_path[len(mountSource)] != filepath.Separator {
				continue
			}

			relPath, err := filepath.Rel(mountSource, converted_path)
			if err != nil {
				logrus.Debugf("Failed to get relative path: %v", err)
				continue
			}
			target := filepath.Join(mount.Target, relPath)
			return &LocalAPIMap{
				ClientPath: pathABS,
				RemotePath: target,
			}, true
		}
	}
	return nil, false
}

// CheckPathOnRunningMachine is a convenience function that checks if a path is available
// on any currently running machine. It combines machine inspection and path checking.
func CheckPathOnRunningMachine(ctx context.Context, path string) (*LocalAPIMap, bool) {
	if err := fileutils.Exists(path); errors.Is(err, fs.ErrNotExist) {
		logrus.Debugf("Path %s does not exist locally, skipping machine check", path)
		return nil, false
	}

	if machineMode := bindings.GetMachineMode(ctx); !machineMode {
		logrus.Debug("Machine mode is not enabled, skipping machine check")
		return nil, false
	}

	conn, err := bindings.GetClient(ctx)
	if err != nil {
		logrus.Debugf("Failed to get client connection: %v", err)
		return nil, false
	}

	machineProvider, err := provider.Get()
	if err != nil {
		return nil, false
	}

	mounts, err := getMachineMounts(conn.URI.String(), conn.URI, machineProvider)
	if err != nil {
		logrus.Debugf("Failed to get machine mounts: %v", err)
		return nil, false
	}

	return isPathAvailableOnMachine(mounts, machineProvider.VMType(), path)
}
