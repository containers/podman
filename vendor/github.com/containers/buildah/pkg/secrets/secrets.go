package secrets

import (
	"bufio"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/containers/storage/pkg/idtools"
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
		if strings.HasPrefix(strings.TrimSpace(scanner.Text()), "/") {
			mounts = append(mounts, scanner.Text())
		} else {
			logrus.Debugf("skipping unrecognized mount in %v: %q",
				filePath, scanner.Text())
		}
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
func SecretMounts(mountLabel, containerWorkingDir, mountFile string, rootless bool) []rspec.Mount {
	return SecretMountsWithUIDGID(mountLabel, containerWorkingDir, mountFile, containerWorkingDir, 0, 0, rootless)
}

// SecretMountsWithUIDGID specifies the uid/gid of the owner
func SecretMountsWithUIDGID(mountLabel, containerWorkingDir, mountFile, mountPrefix string, uid, gid int, rootless bool) []rspec.Mount {
	var (
		secretMounts []rspec.Mount
		mountFiles   []string
	)
	// Add secrets from paths given in the mounts.conf files
	// mountFile will have a value if the hidden --default-mounts-file flag is set
	// Note for testing purposes only
	if mountFile == "" {
		mountFiles = append(mountFiles, []string{OverrideMountsFile, DefaultMountsFile}...)
		if rootless {
			mountFiles = append([]string{UserOverrideMountsFile}, mountFiles...)
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
		if err := addFIPSModeSecret(&secretMounts, containerWorkingDir, mountPrefix, mountLabel, uid, gid); err != nil {
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
		hostDirOrFile, ctrDirOrFile, err := getMountsMap(path)
		if err != nil {
			return nil, err
		}
		// skip if the hostDirOrFile path doesn't exist
		fileInfo, err := os.Stat(hostDirOrFile)
		if err != nil {
			if os.IsNotExist(err) {
				logrus.Warnf("Path %q from %q doesn't exist, skipping", hostDirOrFile, filePath)
				continue
			}
			return nil, errors.Wrapf(err, "failed to stat %q", hostDirOrFile)
		}

		ctrDirOrFileOnHost := filepath.Join(containerWorkingDir, ctrDirOrFile)

		// In the event of a restart, don't want to copy secrets over again as they already would exist in ctrDirOrFileOnHost
		_, err = os.Stat(ctrDirOrFileOnHost)
		if os.IsNotExist(err) {

			hostDirOrFile, err = resolveSymbolicLink(hostDirOrFile)
			if err != nil {
				return nil, err
			}

			switch mode := fileInfo.Mode(); {
			case mode.IsDir():
				if err = os.MkdirAll(ctrDirOrFileOnHost, 0755); err != nil {
					return nil, errors.Wrapf(err, "making container directory %q failed", ctrDirOrFileOnHost)
				}
				data, err := getHostSecretData(hostDirOrFile)
				if err != nil {
					return nil, errors.Wrapf(err, "getting host secret data failed")
				}
				for _, s := range data {
					if err := s.saveTo(ctrDirOrFileOnHost); err != nil {
						return nil, errors.Wrapf(err, "error saving data to container filesystem on host %q", ctrDirOrFileOnHost)
					}
				}
			case mode.IsRegular():
				data, err := readFile("", hostDirOrFile)
				if err != nil {
					return nil, errors.Wrapf(err, "error reading file %q", hostDirOrFile)

				}
				for _, s := range data {
					if err := os.MkdirAll(filepath.Dir(ctrDirOrFileOnHost), 0700); err != nil {
						return nil, err
					}
					if err := ioutil.WriteFile(ctrDirOrFileOnHost, s.data, 0700); err != nil {
						return nil, errors.Wrapf(err, "error saving data to container filesystem on host %q", ctrDirOrFileOnHost)
					}
				}
			default:
				return nil, errors.Errorf("unsupported file type for: %q", hostDirOrFile)
			}

			err = label.Relabel(ctrDirOrFileOnHost, mountLabel, false)
			if err != nil {
				return nil, errors.Wrap(err, "error applying correct labels")
			}
			if uid != 0 || gid != 0 {
				if err := rchown(ctrDirOrFileOnHost, uid, gid); err != nil {
					return nil, err
				}
			}
		} else if err != nil {
			return nil, errors.Wrapf(err, "error getting status of %q", ctrDirOrFileOnHost)
		}

		m := rspec.Mount{
			Source:      filepath.Join(mountPrefix, ctrDirOrFile),
			Destination: ctrDirOrFile,
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
func addFIPSModeSecret(mounts *[]rspec.Mount, containerWorkingDir, mountPrefix, mountLabel string, uid, gid int) error {
	secretsDir := "/run/secrets"
	ctrDirOnHost := filepath.Join(containerWorkingDir, secretsDir)
	if _, err := os.Stat(ctrDirOnHost); os.IsNotExist(err) {
		if err = idtools.MkdirAllAs(ctrDirOnHost, 0755, uid, gid); err != nil {
			return errors.Wrapf(err, "making container directory on host failed")
		}
		if err = label.Relabel(ctrDirOnHost, mountLabel, false); err != nil {
			return errors.Wrap(err, "error applying correct labels")
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
			Source:      filepath.Join(mountPrefix, secretsDir),
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
