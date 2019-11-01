package libpodruntime

import (
	"context"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/cgroups"
	"github.com/containers/libpod/pkg/namespaces"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/containers/libpod/pkg/util"
	"github.com/containers/storage"
	"github.com/pkg/errors"
)

// GetRuntimeMigrate gets a libpod runtime that will perform a migration of existing containers
func GetRuntimeMigrate(ctx context.Context, c *cliconfig.PodmanCommand, newRuntime string) (*libpod.Runtime, error) {
	return getRuntime(ctx, c, false, true, false, true, newRuntime)
}

// GetRuntimeDisableFDs gets a libpod runtime that will disable sd notify
func GetRuntimeDisableFDs(ctx context.Context, c *cliconfig.PodmanCommand) (*libpod.Runtime, error) {
	return getRuntime(ctx, c, false, false, false, false, "")
}

// GetRuntimeRenumber gets a libpod runtime that will perform a lock renumber
func GetRuntimeRenumber(ctx context.Context, c *cliconfig.PodmanCommand) (*libpod.Runtime, error) {
	return getRuntime(ctx, c, true, false, false, true, "")
}

// GetRuntime generates a new libpod runtime configured by command line options
func GetRuntime(ctx context.Context, c *cliconfig.PodmanCommand) (*libpod.Runtime, error) {
	return getRuntime(ctx, c, false, false, false, true, "")
}

// GetRuntimeNoStore generates a new libpod runtime configured by command line options
func GetRuntimeNoStore(ctx context.Context, c *cliconfig.PodmanCommand) (*libpod.Runtime, error) {
	return getRuntime(ctx, c, false, false, true, true, "")
}

func getRuntime(ctx context.Context, c *cliconfig.PodmanCommand, renumber, migrate, noStore, withFDS bool, newRuntime string) (*libpod.Runtime, error) {
	options := []libpod.RuntimeOption{}
	storageOpts := storage.StoreOptions{}
	storageSet := false

	uidmapFlag := c.Flags().Lookup("uidmap")
	gidmapFlag := c.Flags().Lookup("gidmap")
	subuidname := c.Flags().Lookup("subuidname")
	subgidname := c.Flags().Lookup("subgidname")
	if (uidmapFlag != nil && gidmapFlag != nil && subuidname != nil && subgidname != nil) &&
		(uidmapFlag.Changed || gidmapFlag.Changed || subuidname.Changed || subgidname.Changed) {
		userns, _ := c.Flags().GetString("userns")
		uidmapVal, _ := c.Flags().GetStringSlice("uidmap")
		gidmapVal, _ := c.Flags().GetStringSlice("gidmap")
		subuidVal, _ := c.Flags().GetString("subuidname")
		subgidVal, _ := c.Flags().GetString("subgidname")
		mappings, err := util.ParseIDMapping(namespaces.UsernsMode(userns), uidmapVal, gidmapVal, subuidVal, subgidVal)
		if err != nil {
			return nil, err
		}
		storageOpts.UIDMap = mappings.UIDMap
		storageOpts.GIDMap = mappings.GIDMap

		storageSet = true
	}

	if c.Flags().Changed("root") {
		storageSet = true
		storageOpts.GraphRoot = c.GlobalFlags.Root
	}
	if c.Flags().Changed("runroot") {
		storageSet = true
		storageOpts.RunRoot = c.GlobalFlags.Runroot
	}
	if len(storageOpts.RunRoot) > 50 {
		return nil, errors.New("the specified runroot is longer than 50 characters")
	}
	if c.Flags().Changed("storage-driver") {
		storageSet = true
		storageOpts.GraphDriverName = c.GlobalFlags.StorageDriver
		// Overriding the default storage driver caused GraphDriverOptions from storage.conf to be ignored
		storageOpts.GraphDriverOptions = []string{}
	}
	// This should always be checked after storage-driver is checked
	if len(c.GlobalFlags.StorageOpts) > 0 {
		storageSet = true
		storageOpts.GraphDriverOptions = c.GlobalFlags.StorageOpts
	}
	if migrate {
		options = append(options, libpod.WithMigrate())
		if newRuntime != "" {
			options = append(options, libpod.WithMigrateRuntime(newRuntime))
		}
	}

	if renumber {
		options = append(options, libpod.WithRenumber())
	}

	// Only set this if the user changes storage config on the command line
	if storageSet {
		options = append(options, libpod.WithStorageConfig(storageOpts))
	}

	if !storageSet && noStore {
		options = append(options, libpod.WithNoStore())
	}
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
	if c.Flags().Changed("network-cmd-path") {
		options = append(options, libpod.WithNetworkCmdPath(c.GlobalFlags.NetworkCmdPath))
	}

	if c.Flags().Changed("events-backend") {
		options = append(options, libpod.WithEventsLogger(c.GlobalFlags.EventsBackend))
	}

	if c.Flags().Changed("cgroup-manager") {
		options = append(options, libpod.WithCgroupManager(c.GlobalFlags.CGroupManager))
	} else {
		unified, err := cgroups.IsCgroup2UnifiedMode()
		if err != nil {
			return nil, err
		}
		if rootless.IsRootless() && !unified {
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

	// TODO I don't think these belong here?
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

	if !withFDS {
		options = append(options, libpod.WithEnableSDNotify())
	}
	if c.Flags().Changed("config") {
		return libpod.NewRuntimeFromConfig(ctx, c.GlobalFlags.Config, options...)
	}
	return libpod.NewRuntime(ctx, options...)
}
