package libpodruntime

import (
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/containers/libpod/pkg/util"
	"github.com/pkg/errors"
)

// GetRuntime generates a new libpod runtime configured by command line options
func GetRuntime(c *cliconfig.PodmanCommand) (*libpod.Runtime, error) {
	options := []libpod.RuntimeOption{}

	storageOpts, volumePath, err := util.GetDefaultStoreOptions()
	if err != nil {
		return nil, err
	}

	uidmapFlag := c.Flags().Lookup("uidmap")
	gidmapFlag := c.Flags().Lookup("gidmap")
	subuidname := c.Flags().Lookup("subuidname")
	subgidname := c.Flags().Lookup("subgidname")
	if (uidmapFlag != nil && gidmapFlag != nil && subuidname != nil && subgidname != nil) &&
		(uidmapFlag.Changed || gidmapFlag.Changed || subuidname.Changed || subgidname.Changed) {
		uidmapVal, _ := c.Flags().GetStringSlice("uidmap")
		gidmapVal, _ := c.Flags().GetStringSlice("gidmap")
		subuidVal, _ := c.Flags().GetString("subuidname")
		subgidVal, _ := c.Flags().GetString("subgidname")
		mappings, err := util.ParseIDMapping(uidmapVal, gidmapVal, subuidVal, subgidVal)
		if err != nil {
			return nil, err
		}
		storageOpts.UIDMap = mappings.UIDMap
		storageOpts.GIDMap = mappings.GIDMap

	}

	if c.Flags().Changed("root") {
		storageOpts.GraphRoot = c.GlobalFlags.Root
	}
	if c.Flags().Changed("runroot") {
		storageOpts.RunRoot = c.GlobalFlags.Runroot
	}
	if len(storageOpts.RunRoot) > 50 {
		return nil, errors.New("the specified runroot is longer than 50 characters")
	}
	if c.Flags().Changed("storage-driver") {
		storageOpts.GraphDriverName = c.GlobalFlags.StorageDriver
	}
	if len(c.GlobalFlags.StorageOpts) > 0 {
		storageOpts.GraphDriverOptions = c.GlobalFlags.StorageOpts
	}

	options = append(options, libpod.WithStorageConfig(storageOpts))

	// TODO CLI flags for image config?
	// TODO CLI flag for signature policy?

	if len(c.GlobalFlags.Namespace) > 0 {
		options = append(options, libpod.WithNamespace(c.GlobalFlags.Namespace))
	}

	if c.Flags().Changed("runtime") {
		options = append(options, libpod.WithOCIRuntime(c.GlobalFlags.Runtime))
	}

	if c.Flags().Changed("conmon") {
		options = append(options, libpod.WithConmonPath(c.GlobalFlags.ConmonPath))
	}
	if c.Flags().Changed("tmpdir") {
		options = append(options, libpod.WithTmpDir(c.GlobalFlags.TmpDir))
	}

	if c.Flags().Changed("cgroup-manager") {
		options = append(options, libpod.WithCgroupManager(c.GlobalFlags.CGroupManager))
	} else {
		if rootless.IsRootless() {
			options = append(options, libpod.WithCgroupManager("cgroupfs"))
		}
	}

	// TODO flag to set libpod static dir?
	// TODO flag to set libpod tmp dir?

	if c.Flags().Changed("cni-config-dir") {
		options = append(options, libpod.WithCNIConfigDir(c.GlobalFlags.CniConfigDir))
	}
	if c.Flags().Changed("default-mounts-file") {
		options = append(options, libpod.WithDefaultMountsFile(c.GlobalFlags.DefaultMountsFile))
	}
	if c.Flags().Changed("hooks-dir") {
		options = append(options, libpod.WithHooksDir(c.GlobalFlags.HooksDir...))
	}

	// TODO flag to set CNI plugins dir?

	// TODO I dont think these belong here?
	// Will follow up with a different PR to address
	//
	// Pod create options

	infraImageFlag := c.Flags().Lookup("infra-image")
	if infraImageFlag != nil && infraImageFlag.Changed {
		infraImage, _ := c.Flags().GetString("infra-image")
		options = append(options, libpod.WithDefaultInfraImage(infraImage))
	}

	infraCommandFlag := c.Flags().Lookup("infra-command")
	if infraCommandFlag != nil && infraImageFlag.Changed {
		infraCommand, _ := c.Flags().GetString("infra-command")
		options = append(options, libpod.WithDefaultInfraCommand(infraCommand))
	}
	options = append(options, libpod.WithVolumePath(volumePath))
	if c.Flags().Changed("config") {
		return libpod.NewRuntimeFromConfig(c.GlobalFlags.Config, options...)
	}
	return libpod.NewRuntime(options...)
}
