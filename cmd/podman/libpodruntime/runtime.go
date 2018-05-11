package libpodruntime

import (
	"github.com/containers/storage"
	"github.com/projectatomic/libpod/libpod"
	"github.com/urfave/cli"
)

// GetRuntime generates a new libpod runtime configured by command line options
func GetRuntime(c *cli.Context) (*libpod.Runtime, error) {
	storageOpts := storage.DefaultStoreOptions
	return GetRuntimeWithStorageOpts(c, &storageOpts)
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

	if c.GlobalIsSet("runtime") {
		options = append(options, libpod.WithOCIRuntime(c.GlobalString("runtime")))
	}

	if c.GlobalIsSet("conmon") {
		options = append(options, libpod.WithConmonPath(c.GlobalString("conmon")))
	}

	if c.GlobalIsSet("cgroup-manager") {
		options = append(options, libpod.WithCgroupManager(c.GlobalString("cgroup-manager")))
	}

	// TODO flag to set libpod static dir?
	// TODO flag to set libpod tmp dir?

	if c.GlobalIsSet("cni-config-dir") {
		options = append(options, libpod.WithCNIConfigDir(c.GlobalString("cni-config-dir")))
	}
	if c.GlobalIsSet("default-mounts-file") {
		options = append(options, libpod.WithDefaultMountsFile(c.GlobalString("default-mounts-file")))
	}
	options = append(options, libpod.WithHooksDir(c.GlobalString("hooks-dir-path")))

	// TODO flag to set CNI plugins dir?

	return libpod.NewRuntime(options...)
}
