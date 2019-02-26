package secrets

import (
	"bufio"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/containers/libpod/pkg/rootless"
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
	// UserOverrideMountsFile holds the default mount paths in the form
	// "host_path:container_path" overridden by the rootless user
	UserOverrideMountsFile = filepath.Join(os.Getenv("HOME"), ".config/containers/mounts.conf")
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
		// This is expected on most systems
		logrus.Debugf("file %q not found, skipping...", filePath)
		return nil
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	if err = scanner.Err(); err != nil {
		logrus.Errorf("error reading file %q, %v skipping...", filePath, err)
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

// SecretMounts copies, adds, and mounts the secrets to the container root filesystem
func SecretMounts(mountLabel, containerWorkingDir, mountFile string) []rspec.Mount {
	return SecretMountsWithUIDGID(mountLabel, containerWorkingDir, mountFile, containerWorkingDir, 0, 0)
}

// SecretMountsWithUIDGID specifies the uid/gid of the owner
func SecretMountsWithUIDGID(mountLabel, containerWorkingDir, mountFile, mountPrefix string, uid, gid int) []rspec.Mount {
	var (
		secretMounts []rspec.Mount
		mountFiles   []string
	)
	// Add secrets from paths given in the mounts.conf files
	// mountFile will have a value if the hidden --default-mounts-file flag is set
	// Note for testing purposes only
	if mountFile == "" {
		mountFiles = append(mountFiles, []string{OverrideMountsFile, DefaultMountsFile}...)
		if rootless.IsRootless() {
			mountFiles = append([]string{UserOverrideMountsFile}, mountFiles...)
			_, err := os.Stat(UserOverrideMountsFile)
			if err != nil && os.IsNotExist(err) {
				os.MkdirAll(filepath.Dir(UserOverrideMountsFile), 0755)
				if f, err := os.Create(UserOverrideMountsFile); err != nil {
					logrus.Warnf("could not create file %s: %v", UserOverrideMountsFile, err)
				} else {
					f.Close()
				}
			}
		}
	} else {
		mountFiles = append(mountFiles, mountFile)
	}
	for _, file := range mountFiles {
		if _, err := os.Stat(file); err == nil {
			mounts, err := addSecretsFromMountsFile(file, mountLabel, containerWorkingDir, mountPrefix, uid, gid)
			if err != nil {
				logrus.Warnf("error mounting secrets, skipping: %v", err)
			}
			secretMounts = mounts
			break
		}
	}

	// Add FIPS mode secret if /etc/system-fips exists on the host
	_, err := os.Stat("/etc/system-fips")
	if err == nil {
		if err := addFIPSModeSecret(&secretMounts, containerWorkingDir); err != nil {
			logrus.Errorf("error adding FIPS mode secret to container: %v", err)
		}
	} else if os.IsNotExist(err) {
		logrus.Debug("/etc/system-fips does not exist on host, not mounting FIPS mode secret")
	} else {
		logrus.Errorf("stat /etc/system-fips failed for FIPS mode secret: %v", err)
	}
	return secretMounts
}

func rchown(chowndir string, uid, gid int) error {
	return filepath.Walk(chowndir, func(filePath string, f os.FileInfo, err error) error {
		return os.Lchown(filePath, uid, gid)
	})
}

// addSecretsFromMountsFile copies the contents of host directory to container directory
// and returns a list of mounts
func addSecretsFromMountsFile(filePath, mountLabel, containerWorkingDir, mountPrefix string, uid, gid int) ([]rspec.Mount, error) {
	var mounts []rspec.Mount
	defaultMountsPaths := getMounts(filePath)
	for _, path := range defaultMountsPaths {
		hostDir, ctrDir, err := getMountsMap(path)
		if err != nil {
			return nil, err
		}
		// skip if the hostDir path doesn't exist
		if _, err = os.Stat(hostDir); err != nil {
			if os.IsNotExist(err) {
				logrus.Warnf("Path %q from %q doesn't exist, skipping", hostDir, filePath)
				continue
			}
			return nil, errors.Wrapf(err, "failed to stat %q", hostDir)
		}

		ctrDirOnHost := filepath.Join(containerWorkingDir, ctrDir)

		// In the event of a restart, don't want to copy secrets over again as they already would exist in ctrDirOnHost
		_, err = os.Stat(ctrDirOnHost)
		if os.IsNotExist(err) {
			if err = os.MkdirAll(ctrDirOnHost, 0755); err != nil {
				return nil, errors.Wrapf(err, "making container directory %q failed", ctrDirOnHost)
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
			if uid != 0 || gid != 0 {
				if err := rchown(ctrDirOnHost, uid, gid); err != nil {
					return nil, err
				}
			}
		} else if err != nil {
			return nil, errors.Wrapf(err, "error getting status of %q", ctrDirOnHost)
		}

		m := rspec.Mount{
			Source:      filepath.Join(mountPrefix, ctrDir),
			Destination: ctrDir,
			Type:        "bind",
			Options:     []string{"bind", "rprivate"},
		}

		mounts = append(mounts, m)
	}
	return mounts, nil
}

// addFIPSModeSecret creates /run/secrets/system-fips in the container
// root filesystem if /etc/system-fips exists on hosts.
// This enables the container to be FIPS compliant and run openssl in
// FIPS mode as the host is also in FIPS mode.
func addFIPSModeSecret(mounts *[]rspec.Mount, containerWorkingDir string) error {
	secretsDir := "/run/secrets"
	ctrDirOnHost := filepath.Join(containerWorkingDir, secretsDir)
	if _, err := os.Stat(ctrDirOnHost); os.IsNotExist(err) {
		if err = os.MkdirAll(ctrDirOnHost, 0755); err != nil {
			return errors.Wrapf(err, "making container directory on host failed")
		}
	}
	fipsFile := filepath.Join(ctrDirOnHost, "system-fips")
	// In the event of restart, it is possible for the FIPS mode file to already exist
	if _, err := os.Stat(fipsFile); os.IsNotExist(err) {
		file, err := os.Create(fipsFile)
		if err != nil {
			return errors.Wrapf(err, "error creating system-fips file in container for FIPS mode")
		}
		defer file.Close()
	}

	if !mountExists(*mounts, secretsDir) {
		m := rspec.Mount{
			Source:      ctrDirOnHost,
			Destination: secretsDir,
			Type:        "bind",
			Options:     []string{"bind", "rprivate"},
		}
		*mounts = append(*mounts, m)
	}

	return nil
}

// mountExists checks if a mount already exists in the spec
func mountExists(mounts []rspec.Mount, dest string) bool {
	for _, mount := range mounts {
		if mount.Destination == dest {
			return true
		}
	}
	return false
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
