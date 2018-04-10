package buildah

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	rspec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/selinux/go-selinux/label"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

var (
	// DefaultMountsFile holds the default mount paths in the form
	// "host_path:container_path"
	DefaultMountsFile = "/usr/share/containers/mounts.conf"
	// OverrideMountsFile holds the default mount paths in the form
	// "host_path:container_path" overridden by the user
	OverrideMountsFile = "/etc/containers/mounts.conf"
)

func getMounts(filePath string) []string {
	file, err := os.Open(filePath)
	if err != nil {
		logrus.Warnf("file %q not found, skipping...", filePath)
		return nil
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	if err = scanner.Err(); err != nil {
		logrus.Warnf("error reading file %q, skipping...", filePath)
		return nil
	}
	var mounts []string
	for scanner.Scan() {
		mounts = append(mounts, scanner.Text())
	}
	return mounts
}

// getHostAndCtrDir separates the host:container paths
func getMountsMap(path string) (string, string, error) {
	arr := strings.SplitN(path, ":", 2)
	if len(arr) == 2 {
		return arr[0], arr[1], nil
	}
	return "", "", errors.Errorf("unable to get host and container dir")
}

// secretMount copies the contents of host directory to container directory
// and returns a list of mounts
func secretMounts(filePath, mountLabel, containerWorkingDir string) ([]rspec.Mount, error) {
	var mounts []rspec.Mount
	defaultMountsPaths := getMounts(filePath)
	for _, path := range defaultMountsPaths {
		hostDir, ctrDir, err := getMountsMap(path)
		if err != nil {
			return nil, err
		}
		// skip if the hostDir path doesn't exist
		if _, err = os.Stat(hostDir); os.IsNotExist(err) {
			logrus.Warnf("%q doesn't exist, skipping", hostDir)
			continue
		}

		ctrDirOnHost := filepath.Join(containerWorkingDir, ctrDir)
		if err = os.RemoveAll(ctrDirOnHost); err != nil {
			return nil, fmt.Errorf("remove container directory failed: %v", err)
		}

		if err = os.MkdirAll(ctrDirOnHost, 0755); err != nil {
			return nil, fmt.Errorf("making container directory failed: %v", err)
		}

		hostDir, err = resolveSymbolicLink(hostDir)
		if err != nil {
			return nil, err
		}

		if err = copyWithTar(hostDir, ctrDirOnHost); err != nil && !os.IsNotExist(err) {
			return nil, errors.Wrapf(err, "error getting host secret data")
		}

		err = label.Relabel(ctrDirOnHost, mountLabel, false)
		if err != nil {
			return nil, errors.Wrap(err, "error applying correct labels")
		}

		m := rspec.Mount{
			Source:      ctrDirOnHost,
			Destination: ctrDir,
			Type:        "bind",
			Options:     []string{"bind"},
		}

		mounts = append(mounts, m)
	}
	return mounts, nil
}

// resolveSymbolicLink resolves a possbile symlink path. If the path is a symlink, returns resolved
// path; if not, returns the original path.
func resolveSymbolicLink(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != os.ModeSymlink {
		return path, nil
	}
	return filepath.EvalSymlinks(path)
}
