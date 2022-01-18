// +build !remote

package infra

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"

	"github.com/containers/common/pkg/cgroups"
	"github.com/containers/podman/v4/cmd/podman/utils"
	"github.com/containers/podman/v4/libpod"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/namespaces"
	"github.com/containers/podman/v4/pkg/rootless"
	"github.com/containers/storage/pkg/idtools"
	"github.com/containers/storage/types"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	flag "github.com/spf13/pflag"
)

var (
	// runtimeSync only guards the non-specialized runtime
	runtimeSync sync.Once
	// The default GetRuntime() always returns the same object and error
	runtimeLib *libpod.Runtime
	runtimeErr error
)

type engineOpts struct {
	name     string
	renumber bool
	migrate  bool
	noStore  bool
	withFDS  bool
	config   *entities.PodmanConfig
}

// GetRuntimeMigrate gets a libpod runtime that will perform a migration of existing containers
func GetRuntimeMigrate(ctx context.Context, fs *flag.FlagSet, cfg *entities.PodmanConfig, newRuntime string) (*libpod.Runtime, error) {
	return getRuntime(ctx, fs, &engineOpts{
		name:     newRuntime,
		renumber: false,
		migrate:  true,
		noStore:  false,
		withFDS:  true,
		config:   cfg,
	})
}

// GetRuntimeDisableFDs gets a libpod runtime that will disable sd notify
func GetRuntimeDisableFDs(ctx context.Context, fs *flag.FlagSet, cfg *entities.PodmanConfig) (*libpod.Runtime, error) {
	return getRuntime(ctx, fs, &engineOpts{
		renumber: false,
		migrate:  false,
		noStore:  false,
		withFDS:  false,
		config:   cfg,
	})
}

// GetRuntimeRenumber gets a libpod runtime that will perform a lock renumber
func GetRuntimeRenumber(ctx context.Context, fs *flag.FlagSet, cfg *entities.PodmanConfig) (*libpod.Runtime, error) {
	return getRuntime(ctx, fs, &engineOpts{
		renumber: true,
		migrate:  false,
		noStore:  false,
		withFDS:  true,
		config:   cfg,
	})
}

// GetRuntime generates a new libpod runtime configured by command line options
func GetRuntime(ctx context.Context, flags *flag.FlagSet, cfg *entities.PodmanConfig) (*libpod.Runtime, error) {
	runtimeSync.Do(func() {
		runtimeLib, runtimeErr = getRuntime(ctx, flags, &engineOpts{
			renumber: false,
			migrate:  false,
			noStore:  false,
			withFDS:  true,
			config:   cfg,
		})
	})
	return runtimeLib, runtimeErr
}

// GetRuntimeNoStore generates a new libpod runtime configured by command line options
func GetRuntimeNoStore(ctx context.Context, fs *flag.FlagSet, cfg *entities.PodmanConfig) (*libpod.Runtime, error) {
	return getRuntime(ctx, fs, &engineOpts{
		renumber: false,
		migrate:  false,
		noStore:  true,
		withFDS:  true,
		config:   cfg,
	})
}

func getRuntime(ctx context.Context, fs *flag.FlagSet, opts *engineOpts) (*libpod.Runtime, error) {
	options := []libpod.RuntimeOption{}
	storageOpts := types.StoreOptions{}
	cfg := opts.config

	storageSet := false

	uidmapFlag := fs.Lookup("uidmap")
	gidmapFlag := fs.Lookup("gidmap")
	subuidname := fs.Lookup("subuidname")
	subgidname := fs.Lookup("subgidname")
	if (uidmapFlag != nil && gidmapFlag != nil && subuidname != nil && subgidname != nil) &&
		(uidmapFlag.Changed || gidmapFlag.Changed || subuidname.Changed || subgidname.Changed) {
		userns, _ := fs.GetString("userns")
		uidmapVal, _ := fs.GetStringSlice("uidmap")
		gidmapVal, _ := fs.GetStringSlice("gidmap")
		subuidVal, _ := fs.GetString("subuidname")
		subgidVal, _ := fs.GetString("subgidname")
		mappings, err := ParseIDMapping(namespaces.UsernsMode(userns), uidmapVal, gidmapVal, subuidVal, subgidVal)
		if err != nil {
			return nil, err
		}
		storageOpts.UIDMap = mappings.UIDMap
		storageOpts.GIDMap = mappings.GIDMap

		storageSet = true
	}

	if fs.Changed("root") {
		storageSet = true
		storageOpts.GraphRoot = cfg.Engine.StaticDir
		storageOpts.GraphDriverOptions = []string{}
	}
	if fs.Changed("runroot") {
		storageSet = true
		storageOpts.RunRoot = cfg.Runroot
	}
	if len(storageOpts.RunRoot) > 50 {
		return nil, errors.New("the specified runroot is longer than 50 characters")
	}
	if fs.Changed("storage-driver") {
		storageSet = true
		storageOpts.GraphDriverName = cfg.StorageDriver
		// Overriding the default storage driver caused GraphDriverOptions from storage.conf to be ignored
		storageOpts.GraphDriverOptions = []string{}
	}
	// This should always be checked after storage-driver is checked
	if len(cfg.StorageOpts) > 0 {
		storageSet = true
		if len(cfg.StorageOpts) == 1 && cfg.StorageOpts[0] == "" {
			storageOpts.GraphDriverOptions = []string{}
		} else {
			storageOpts.GraphDriverOptions = cfg.StorageOpts
		}
	}
	if opts.migrate {
		options = append(options, libpod.WithMigrate())
		if opts.name != "" {
			options = append(options, libpod.WithMigrateRuntime(opts.name))
		}
	}

	if opts.renumber {
		options = append(options, libpod.WithRenumber())
	}

	if len(cfg.RuntimeFlags) > 0 {
		runtimeFlags := []string{}
		for _, arg := range cfg.RuntimeFlags {
			runtimeFlags = append(runtimeFlags, "--"+arg)
		}
		options = append(options, libpod.WithRuntimeFlags(runtimeFlags))
	}

	// Only set this if the user changes storage config on the command line
	if storageSet {
		options = append(options, libpod.WithStorageConfig(storageOpts))
	}

	if !storageSet && opts.noStore {
		options = append(options, libpod.WithNoStore())
	}
	// TODO CLI flags for image config?
	// TODO CLI flag for signature policy?

	if len(cfg.Engine.Namespace) > 0 {
		options = append(options, libpod.WithNamespace(cfg.Engine.Namespace))
	}

	if fs.Changed("runtime") {
		options = append(options, libpod.WithOCIRuntime(cfg.RuntimePath))
	}

	if fs.Changed("conmon") {
		options = append(options, libpod.WithConmonPath(cfg.ConmonPath))
	}
	if fs.Changed("tmpdir") {
		options = append(options, libpod.WithTmpDir(cfg.Engine.TmpDir))
	}
	if fs.Changed("network-cmd-path") {
		options = append(options, libpod.WithNetworkCmdPath(cfg.Engine.NetworkCmdPath))
	}
	if fs.Changed("network-backend") {
		options = append(options, libpod.WithNetworkBackend(cfg.Network.NetworkBackend))
	}

	if fs.Changed("events-backend") {
		options = append(options, libpod.WithEventsLogger(cfg.Engine.EventsLogger))
	}

	if fs.Changed("cgroup-manager") {
		options = append(options, libpod.WithCgroupManager(cfg.Engine.CgroupManager))
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

	if fs.Changed("network-config-dir") {
		options = append(options, libpod.WithCNIConfigDir(cfg.Network.NetworkConfigDir))
	}
	if fs.Changed("default-mounts-file") {
		options = append(options, libpod.WithDefaultMountsFile(cfg.Containers.DefaultMountsFile))
	}
	if fs.Changed("hooks-dir") {
		options = append(options, libpod.WithHooksDir(cfg.Engine.HooksDir...))
	}
	if fs.Changed("registries-conf") {
		options = append(options, libpod.WithRegistriesConf(cfg.RegistriesConf))
	}

	// no need to handle the error, it will return false anyway
	if syslog, _ := fs.GetBool("syslog"); syslog {
		options = append(options, libpod.WithSyslog())
	}

	// TODO flag to set CNI plugins dir?

	if !opts.withFDS {
		options = append(options, libpod.WithEnableSDNotify())
	}
	return libpod.NewRuntime(ctx, options...)
}

// ParseIDMapping takes idmappings and subuid and subgid maps and returns a storage mapping
func ParseIDMapping(mode namespaces.UsernsMode, uidMapSlice, gidMapSlice []string, subUIDMap, subGIDMap string) (*types.IDMappingOptions, error) {
	options := types.IDMappingOptions{
		HostUIDMapping: true,
		HostGIDMapping: true,
	}

	if mode.IsAuto() {
		var err error
		options.HostUIDMapping = false
		options.HostGIDMapping = false
		options.AutoUserNs = true
		opts, err := mode.GetAutoOptions()
		if err != nil {
			return nil, err
		}
		options.AutoUserNsOpts = *opts
		return &options, nil
	}
	if mode.IsKeepID() {
		if len(uidMapSlice) > 0 || len(gidMapSlice) > 0 {
			return nil, errors.New("cannot specify custom mappings with --userns=keep-id")
		}
		if len(subUIDMap) > 0 || len(subGIDMap) > 0 {
			return nil, errors.New("cannot specify subuidmap or subgidmap with --userns=keep-id")
		}
		if rootless.IsRootless() {
			min := func(a, b int) int {
				if a < b {
					return a
				}
				return b
			}

			uid := rootless.GetRootlessUID()
			gid := rootless.GetRootlessGID()

			uids, gids, err := rootless.GetConfiguredMappings()
			if err != nil {
				return nil, errors.Wrapf(err, "cannot read mappings")
			}
			maxUID, maxGID := 0, 0
			for _, u := range uids {
				maxUID += u.Size
			}
			for _, g := range gids {
				maxGID += g.Size
			}

			options.UIDMap, options.GIDMap = nil, nil

			options.UIDMap = append(options.UIDMap, idtools.IDMap{ContainerID: 0, HostID: 1, Size: min(uid, maxUID)})
			options.UIDMap = append(options.UIDMap, idtools.IDMap{ContainerID: uid, HostID: 0, Size: 1})
			if maxUID > uid {
				options.UIDMap = append(options.UIDMap, idtools.IDMap{ContainerID: uid + 1, HostID: uid + 1, Size: maxUID - uid})
			}

			options.GIDMap = append(options.GIDMap, idtools.IDMap{ContainerID: 0, HostID: 1, Size: min(gid, maxGID)})
			options.GIDMap = append(options.GIDMap, idtools.IDMap{ContainerID: gid, HostID: 0, Size: 1})
			if maxGID > gid {
				options.GIDMap = append(options.GIDMap, idtools.IDMap{ContainerID: gid + 1, HostID: gid + 1, Size: maxGID - gid})
			}

			options.HostUIDMapping = false
			options.HostGIDMapping = false
		}
		// Simply ignore the setting and do not setup an inner namespace for root as it is a no-op
		return &options, nil
	}

	if subGIDMap == "" && subUIDMap != "" {
		subGIDMap = subUIDMap
	}
	if subUIDMap == "" && subGIDMap != "" {
		subUIDMap = subGIDMap
	}
	if len(gidMapSlice) == 0 && len(uidMapSlice) != 0 {
		gidMapSlice = uidMapSlice
	}
	if len(uidMapSlice) == 0 && len(gidMapSlice) != 0 {
		uidMapSlice = gidMapSlice
	}
	if len(uidMapSlice) == 0 && subUIDMap == "" && os.Getuid() != 0 {
		uidMapSlice = []string{fmt.Sprintf("0:%d:1", os.Getuid())}
	}
	if len(gidMapSlice) == 0 && subGIDMap == "" && os.Getuid() != 0 {
		gidMapSlice = []string{fmt.Sprintf("0:%d:1", os.Getgid())}
	}

	if subUIDMap != "" && subGIDMap != "" {
		mappings, err := idtools.NewIDMappings(subUIDMap, subGIDMap)
		if err != nil {
			return nil, err
		}
		options.UIDMap = mappings.UIDs()
		options.GIDMap = mappings.GIDs()
	}
	parsedUIDMap, err := idtools.ParseIDMap(uidMapSlice, "UID")
	if err != nil {
		return nil, err
	}
	parsedGIDMap, err := idtools.ParseIDMap(gidMapSlice, "GID")
	if err != nil {
		return nil, err
	}
	options.UIDMap = append(options.UIDMap, parsedUIDMap...)
	options.GIDMap = append(options.GIDMap, parsedGIDMap...)
	if len(options.UIDMap) > 0 {
		options.HostUIDMapping = false
	}
	if len(options.GIDMap) > 0 {
		options.HostGIDMapping = false
	}
	return &options, nil
}

// StartWatcher starts a new SIGHUP go routine for the current config.
func StartWatcher(rt *libpod.Runtime) {
	// Setup the signal notifier
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, utils.SIGHUP)

	go func() {
		for {
			// Block until the signal is received
			logrus.Debugf("waiting for SIGHUP to reload configuration")
			<-ch
			if err := rt.Reload(); err != nil {
				logrus.Errorf("Unable to reload configuration: %v", err)
				continue
			}
		}
	}()

	logrus.Debugf("registered SIGHUP watcher for config")
}
