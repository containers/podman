package buildah

import (
	"bufio"
	"fmt"
	"io/ioutil"
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
	// "host_path:container_path" overriden by the user
	OverrideMountsFile = "/etc/containers/mounts.conf"
)

// secretData stores the name of the file and the content read from it
type secretData struct {
	name string
	data []byte
}

// saveTo saves secret data to given directory
func (s secretData) saveTo(dir string) error {
	path := filepath.Join(dir, s.name)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil && !os.IsExist(err) {
		return err
	}
	return ioutil.WriteFile(path, s.data, 0700)
}

func readAll(root, prefix string) ([]secretData, error) {
	path := filepath.Join(root, prefix)

	data := []secretData{}

	files, err := ioutil.ReadDir(path)
	if err != nil {
		if os.IsNotExist(err) {
			return data, nil
		}

		return nil, err
	}

	for _, f := range files {
		fileData, err := readFile(root, filepath.Join(prefix, f.Name()))
		if err != nil {
			// If the file did not exist, might be a dangling symlink
			// Ignore the error
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		data = append(data, fileData...)
	}

	return data, nil
}

func readFile(root, name string) ([]secretData, error) {
	path := filepath.Join(root, name)

	s, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	if s.IsDir() {
		dirData, err := readAll(root, name)
		if err != nil {
			return nil, err
		}
		return dirData, nil
	}
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return []secretData{{name: name, data: bytes}}, nil
}

func getHostSecretData(hostDir string) ([]secretData, error) {
	var allSecrets []secretData
	hostSecrets, err := readAll(hostDir, "")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read secrets from %q", hostDir)
	}
	return append(allSecrets, hostSecrets...), nil
}

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

		data, err := getHostSecretData(hostDir)
		if err != nil {
			return nil, errors.Wrapf(err, "getting host secret data failed")
		}
		for _, s := range data {
			if err := s.saveTo(ctrDirOnHost); err != nil {
				return nil, errors.Wrapf(err, "error saving data to container filesystem on host %q", ctrDirOnHost)
			}
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
