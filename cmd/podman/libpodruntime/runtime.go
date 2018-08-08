package libpodruntime

import (
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/containers/libpod/pkg/util"
	"github.com/containers/storage"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

// GetRuntime generates a new libpod runtime configured by command line options
func GetRuntime(c *cli.Context) (*libpod.Runtime, error) {
	storageOpts := new(storage.StoreOptions)
	options := []libpod.RuntimeOption{}

	_, volumePath, err := util.GetDefaultStoreOptions()
	if err != nil {
		return nil, err
	}

	if c.IsSet("uidmap") || c.IsSet("gidmap") || c.IsSet("subuidmap") || c.IsSet("subgidmap") {
		mappings, err := util.ParseIDMapping(c.StringSlice("uidmap"), c.StringSlice("gidmap"), c.String("subuidmap"), c.String("subgidmap"))
		if err != nil {
			return nil, err
		}
		storageOpts.UIDMap = mappings.UIDMap
		storageOpts.GIDMap = mappings.GIDMap
	}

	if c.GlobalIsSet("root") {
		storageOpts.GraphRoot = c.GlobalString("root")
	}
	if c.GlobalIsSet("runroot") {
		storageOpts.RunRoot = c.GlobalString("runroot")
	}
	if len(storageOpts.RunRoot) > 50 {
		return nil, errors.New("the specified runroot is longer than 50 characters")
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
	if c.GlobalIsSet("hooks-dir") {
		options = append(options, libpod.WithHooksDir(c.GlobalStringSlice("hooks-dir")...))
	}

	// TODO flag to set CNI plugins dir?

	// Pod create options
	if c.IsSet("infra-image") {
		options = append(options, libpod.WithDefaultInfraImage(c.String("infra-image")))
	}

	if c.IsSet("infra-command") {
		options = append(options, libpod.WithDefaultInfraCommand(c.String("infra-command")))
	}
	options = append(options, libpod.WithVolumePath(volumePath))
	if c.IsSet("config") {
		return libpod.NewRuntimeFromConfig(c.String("config"), options...)
	}
	return libpod.NewRuntime(options...)
}
