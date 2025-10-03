//go:build amd64 || arm64

package localapi

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
	"github.com/sirupsen/logrus"
	"go.podman.io/storage/pkg/fileutils"
)

// FindMachineByPort finds a running machine that matches the given connection port.
// It returns the machine configuration and provider, or an error if not found.
func FindMachineByPort(connectionURI string, parsedConnection *url.URL) (*vmconfigs.MachineConfig, vmconfigs.VMProvider, error) {
	for _, machineProvider := range provider.GetAll() {
		logrus.Debugf("Checking provider: %s", machineProvider.VMType())
		dirs, err := env.GetMachineDirs(machineProvider.VMType())
		if err != nil {
			logrus.Debugf("Failed to get machine dirs for provider %s: %v", machineProvider.VMType(), err)
			continue
		}

		machineList, err := vmconfigs.LoadMachinesInDir(dirs)
		if err != nil {
			logrus.Debugf("Failed to list machines: %v", err)
			continue
		}

		// Now we know that the connection points to a machine and we
		// can find the machine by looking for the one with the
		// matching port.
		connectionPort, err := strconv.Atoi(parsedConnection.Port())
		if err != nil {
			logrus.Debugf("Failed to parse connection port: %v", err)
			continue
		}

		for _, mc := range machineList {
			if connectionPort != mc.SSH.Port {
				continue
			}

			state, err := machineProvider.State(mc, false)
			if err != nil {
				logrus.Debugf("Failed to get machine state for %s: %v", mc.Name, err)
				continue
			}

			if state != define.Running {
				return nil, nil, fmt.Errorf("machine %s is not running but in state %s", mc.Name, state)
			}

			return mc, machineProvider, nil
		}
	}

	return nil, nil, fmt.Errorf("could not find a matching machine for connection %q", connectionURI)
}

// getMachineMountsAndVMType retrieves the mounts and VM type of a machine based on the connection URI and parsed URL.
// It returns a slice of mounts, the VM type, or an error if the machine cannot be found or is not running.
func getMachineMountsAndVMType(connectionURI string, parsedConnection *url.URL) ([]*vmconfigs.Mount, define.VMType, error) {
	mc, machineProvider, err := FindMachineByPort(connectionURI, parsedConnection)
	if err != nil {
		return nil, define.UnknownVirt, err
	}
	return mc.Mounts, machineProvider.VMType(), nil
}

// isPathAvailableOnMachine checks if a local path is available on the machine through mounted directories.
// If the path is available, it returns a LocalAPIMap with the corresponding remote path.
func isPathAvailableOnMachine(mounts []*vmconfigs.Mount, vmType define.VMType, path string) (*LocalAPIMap, bool) {
	pathABS, err := filepath.Abs(path)
	if err != nil {
		logrus.Debugf("Failed to get absolute path for %s: %v", path, err)
		return nil, false
	}

	// WSLVirt is a special case where there is no real concept of doing a mount in WSL,
	// WSL by default mounts the drives to /mnt/c, /mnt/d, etc...
	if vmType == define.WSLVirt {
		converted_path, err := specgen.ConvertWinMountPath(pathABS)
		if err != nil {
			logrus.Debugf("Failed to convert Windows mount path: %v", err)
			return nil, false
		}

		return &LocalAPIMap{
			ClientPath: pathABS,
			RemotePath: converted_path,
		}, true
	}

	for _, mount := range mounts {
		mountSource := filepath.Clean(mount.Source)
		relPath, err := filepath.Rel(mountSource, pathABS)
		if err != nil {
			logrus.Debugf("Failed to get relative path: %v", err)
			continue
		}
		// If relPath starts with ".." or is absolute, pathABS is not under mountSource
		if relPath == "." || (!strings.HasPrefix(relPath, "..") && !filepath.IsAbs(relPath)) {
			target := filepath.Join(mount.Target, relPath)
			converted_path, err := specgen.ConvertWinMountPath(target)
			if err != nil {
				logrus.Debugf("Failed to convert Windows mount path: %v", err)
				return nil, false
			}
			logrus.Debugf("Converted client path: %q", converted_path)
			return &LocalAPIMap{
				ClientPath: pathABS,
				RemotePath: converted_path,
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

	mounts, vmType, err := getMachineMountsAndVMType(conn.URI.String(), conn.URI)
	if err != nil {
		logrus.Debugf("Failed to get machine mounts: %v", err)
		return nil, false
	}

	return isPathAvailableOnMachine(mounts, vmType, path)
}

// CheckMultiplePathsOnRunningMachine checks if multiple paths are available on a running machine.
// Returns a translation map of local-to-remote paths if all paths are accessible, false otherwise.
func CheckMultiplePathsOnRunningMachine(ctx context.Context, paths []string) (TranslationLocalAPIMap, bool) {
	result := TranslationLocalAPIMap{}

	if machineMode := bindings.GetMachineMode(ctx); !machineMode {
		logrus.Debug("Machine mode is not enabled, skipping machine check")
		return nil, false
	}

	conn, err := bindings.GetClient(ctx)
	if err != nil {
		logrus.Debugf("Failed to get client connection: %v", err)
		return nil, false
	}

	mounts, vmType, err := getMachineMountsAndVMType(conn.URI.String(), conn.URI)
	if err != nil {
		logrus.Debugf("Failed to get machine mounts: %v", err)
		return nil, false
	}

	for _, path := range paths {
		if err := fileutils.Exists(path); errors.Is(err, fs.ErrNotExist) {
			logrus.Debugf("Path %s does not exist locally, skipping machine check", path)
			return nil, false
		}
		mapping, found := isPathAvailableOnMachine(mounts, vmType, path)
		if !found {
			logrus.Debugf("Path %s is not available on the running machine", path)
			return nil, false
		}
		result[path] = mapping.RemotePath
	}

	logrus.Debugf("Successfully created local-to-remote path translation map: %#v\n", result)
	return result, true
}

func IsHyperVProvider(ctx context.Context) (bool, error) {
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		logrus.Debugf("Failed to get client connection: %v", err)
		return false, err
	}

	_, vmProvider, err := FindMachineByPort(conn.URI.String(), conn.URI)
	if err != nil {
		logrus.Debugf("Failed to get machine hypervisor type: %v", err)
		return false, err
	}

	return vmProvider.VMType() == define.HyperVVirt, nil
}

// ValidatePathForLocalAPI checks if the provided path satisfies requirements for local API usage.
// It returns an error if the path is not absolute or does not exist on the filesystem.
func ValidatePathForLocalAPI(path string) error {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("path %q is not absolute", path)
	}

	if err := fileutils.Exists(path); err != nil {
		return err
	}
	return nil
}
