package libpodruntime

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/containers/storage"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

// GetRuntime generates a new libpod runtime configured by command line options
func GetRuntime(c *cli.Context) (*libpod.Runtime, error) {
	storageOpts, err := GetDefaultStoreOptions()
	if err != nil {
		return nil, err
	}
	return GetRuntimeWithStorageOpts(c, &storageOpts)
}

func GetRootlessStorageOpts() (storage.StoreOptions, error) {
	var opts storage.StoreOptions

	rootlessRuntime, err := libpod.GetRootlessRuntimeDir()
	if err != nil {
		return opts, err
	}
	opts.RunRoot = filepath.Join(rootlessRuntime, "run")

	dataDir := os.Getenv("XDG_DATA_HOME")
	if dataDir == "" {
		home := os.Getenv("HOME")
		if home == "" {
			return opts, fmt.Errorf("neither XDG_DATA_HOME nor HOME was set non-empty")
		}
		// runc doesn't like symlinks in the rootfs path, and at least
		// on CoreOS /home is a symlink to /var/home, so resolve any symlink.
		resolvedHome, err := filepath.EvalSymlinks(home)
		if err != nil {
			return opts, errors.Wrapf(err, "cannot resolve %s", home)
		}
		dataDir = filepath.Join(resolvedHome, ".local", "share")
	}
	opts.GraphRoot = filepath.Join(dataDir, "containers", "storage")
	opts.GraphDriverName = "vfs"
	return opts, nil
}

func GetDefaultStoreOptions() (storage.StoreOptions, error) {
	storageOpts := storage.DefaultStoreOptions
	if rootless.IsRootless() {
		var err error
		storageOpts, err = GetRootlessStorageOpts()
		if err != nil {
			return storageOpts, err
		}

		storageConf := filepath.Join(os.Getenv("HOME"), ".config/containers/storage.conf")
		if _, err := os.Stat(storageConf); err == nil {
			storage.ReloadConfigurationFile(storageConf, &storageOpts)
		}
	}
	return storageOpts, nil
}

// GetRuntime generates a new libpod runtime configured by command line options
func GetRuntimeWithStorageOpts(c *cli.Context, storageOpts *storage.StoreOptions) (*libpod.Runtime, error) {
	options := []libpod.RuntimeOption{}

	if c.GlobalIsSet("root") {
		storageOpts.GraphRoot = c.GlobalString("root")
	}
	if c.GlobalIsSet("runroot") {
		storageOpts.RunRoot = c.GlobalString("runroot")
	}
	if c.GlobalIsSet("storage-driver") {
		storageOpts.GraphDriverName = c.GlobalString("storage-driver")
	}
	if c.GlobalIsSet("storage-opt") {
		storageOpts.GraphDriverOptions = c.GlobalStringSlice("storage-opt")
	}

	options = append(options, libpod.WithStorageConfig(*storageOpts))

	// TODO CLI flags for image config?
	// TODO CLI flag for signature policy?

	if c.GlobalIsSet("namespace") {
		options = append(options, libpod.WithNamespace(c.GlobalString("namespace")))
	}

	if c.GlobalIsSet("runtime") {
		options = append(options, libpod.WithOCIRuntime(c.GlobalString("runtime")))
	}

	if c.GlobalIsSet("conmon") {
		options = append(options, libpod.WithConmonPath(c.GlobalString("conmon")))
	}
	if c.GlobalIsSet("tmpdir") {
		options = append(options, libpod.WithTmpDir(c.GlobalString("tmpdir")))
	}

	if c.GlobalIsSet("cgroup-manager") {
		options = append(options, libpod.WithCgroupManager(c.GlobalString("cgroup-manager")))
	} else {
		if rootless.IsRootless() {
			options = append(options, libpod.WithCgroupManager("cgroupfs"))
		}
	}

	// TODO flag to set libpod static dir?
	// TODO flag to set libpod tmp dir?

	if c.GlobalIsSet("cni-config-dir") {
		options = append(options, libpod.WithCNIConfigDir(c.GlobalString("cni-config-dir")))
	}
	if c.GlobalIsSet("default-mounts-file") {
		options = append(options, libpod.WithDefaultMountsFile(c.GlobalString("default-mounts-file")))
	}
	options = append(options, libpod.WithHooksDir(c.GlobalString("hooks-dir-path"), c.GlobalIsSet("hooks-dir-path")))

	// TODO flag to set CNI plugins dir?

	// Pod create options
	if c.IsSet("pause-image") {
		options = append(options, libpod.WithDefaultPauseImage(c.String("pause-image")))
	}

	if c.IsSet("pause-command") {
		options = append(options, libpod.WithDefaultPauseCommand(c.String("pause-command")))
	}

	return libpod.NewRuntime(options...)
}
