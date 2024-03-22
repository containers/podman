//go:build !remote

package libpod

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/unix"

	"github.com/containers/common/pkg/cgroups"
	"github.com/containers/podman/v5/pkg/rootless"
	"github.com/containers/podman/v5/pkg/systemd"
	"github.com/sirupsen/logrus"
)

const TmpfsWarningEnvVar = "PODMAN_SUPPRESS_TMPFS_WARNING"

func checkCgroups2UnifiedMode(runtime *Runtime) {
	unified, _ := cgroups.IsCgroup2UnifiedMode()
	// DELETE ON RHEL9
	if !unified {
		_, ok := os.LookupEnv("PODMAN_IGNORE_CGROUPSV1_WARNING")
		if !ok {
			logrus.Warn("Using cgroups-v1 which is deprecated in favor of cgroups-v2 with Podman v5 and will be removed in a future version. Set environment variable `PODMAN_IGNORE_CGROUPSV1_WARNING` to hide this warning.")
		}
	}
	// DELETE ON RHEL9

	if unified && rootless.IsRootless() && !systemd.IsSystemdSessionValid(rootless.GetRootlessUID()) {
		// If user is rootless and XDG_RUNTIME_DIR is found, podman will not proceed with /tmp directory
		// it will try to use existing XDG_RUNTIME_DIR
		// if current user has no write access to XDG_RUNTIME_DIR we will fail later
		if err := unix.Access(runtime.storageConfig.RunRoot, unix.W_OK); err != nil {
			msg := fmt.Sprintf("RunRoot is pointing to a path (%s) which is not writable. Most likely podman will fail.", runtime.storageConfig.RunRoot)
			if errors.Is(err, os.ErrNotExist) {
				// if dir does not exist, try to create it
				if err := os.MkdirAll(runtime.storageConfig.RunRoot, 0700); err != nil {
					logrus.Warn(msg)
				}
			} else {
				logrus.Warnf("%s: %v", msg, err)
			}
		}
	}
}

// Print a warning if a path is not on a tmpfs.
func warnIfNotTmpfs(paths []string) {
	if os.Getenv(TmpfsWarningEnvVar) != "" {
		return
	}

	notTmpfs := make([]string, 0, len(paths))

	for _, path := range paths {
		statfs := new(unix.Statfs_t)
		if err := unix.Statfs(path, statfs); err != nil {
			return
		}

		if statfs.Type != unix.TMPFS_MAGIC && uint64(statfs.Type) != unix.RAMFS_MAGIC {
			notTmpfs = append(notTmpfs, path)
		}
	}

	if len(notTmpfs) == 0 {
		return
	}

	logrus.Warn("The following temporary files directories are not on a tmpfs filesystem:")
	for _, path := range notTmpfs {
		logrus.Warnf("     %q", path)
	}
	logrus.Warn("This may cause unpredictable behavior as Podman cannot properly detect and react to system reboots.")
	logrus.Warn("It is recommended that you change the path used to one on a tmpfs or mount a tmpfs on the given path.")
	logrus.Warnf("If you are certain your configuration is safe and just want to suppress this warning, set the %s environment variable to any non-empty value.", TmpfsWarningEnvVar)
}
