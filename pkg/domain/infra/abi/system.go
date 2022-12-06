package abi

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/containers/common/pkg/cgroups"
	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/domain/entities/reports"
	"github.com/containers/podman/v4/pkg/rootless"
	"github.com/containers/podman/v4/pkg/util"
	"github.com/containers/podman/v4/utils"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/unshare"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
)

func (ic *ContainerEngine) Info(ctx context.Context) (*define.Info, error) {
	info, err := ic.Libpod.Info()
	if err != nil {
		return nil, err
	}
	info.Host.RemoteSocket = &define.RemoteSocket{Path: ic.Libpod.RemoteURI()}

	// `podman system connection add` invokes podman via ssh to fill in connection string. Here
	// we are reporting the default systemd activation socket path as we cannot know if a future
	// service may be run with another URI.
	if ic.Libpod.RemoteURI() == "" {
		xdg := "/run"
		if path, err := util.GetRuntimeDir(); err != nil {
			// Info is as good as we can guess...
			return info, err
		} else if path != "" {
			xdg = path
		}

		uri := url.URL{
			Scheme: "unix",
			Path:   filepath.Join(xdg, "podman", "podman.sock"),
		}
		ic.Libpod.SetRemoteURI(uri.String())
		info.Host.RemoteSocket.Path = uri.Path
	}

	uri, err := url.Parse(ic.Libpod.RemoteURI())
	if err != nil {
		return nil, err
	}

	if uri.Scheme == "unix" {
		_, err := os.Stat(uri.Path)
		info.Host.RemoteSocket.Exists = err == nil
	} else {
		info.Host.RemoteSocket.Exists = true
	}

	return info, err
}

func (ic *ContainerEngine) SetupRootless(_ context.Context, noMoveProcess bool) error {
	runsUnderSystemd := utils.RunsOnSystemd()
	if !runsUnderSystemd {
		isPid1 := os.Getpid() == 1
		if _, found := os.LookupEnv("container"); isPid1 || found {
			if err := utils.MaybeMoveToSubCgroup(); err != nil {
				// it is a best effort operation, so just print the
				// error for debugging purposes.
				logrus.Debugf("Could not move to subcgroup: %v", err)
			}
		}
	}

	if !rootless.IsRootless() {
		return nil
	}

	// do it only after podman has already re-execed and running with uid==0.
	hasCapSysAdmin, err := unshare.HasCapSysAdmin()
	if err != nil {
		return err
	}
	if hasCapSysAdmin {
		ownsCgroup, err := cgroups.UserOwnsCurrentSystemdCgroup()
		if err != nil {
			logrus.Infof("Failed to detect the owner for the current cgroup: %v", err)
		}
		if !ownsCgroup {
			conf, err := ic.Config(context.Background())
			if err != nil {
				return err
			}
			unitName := fmt.Sprintf("podman-%d.scope", os.Getpid())
			if runsUnderSystemd || conf.Engine.CgroupManager == config.SystemdCgroupsManager {
				if err := utils.RunUnderSystemdScope(os.Getpid(), "user.slice", unitName); err != nil {
					logrus.Debugf("Failed to add podman to systemd sandbox cgroup: %v", err)
				}
			}
		}
		return nil
	}

	tmpDir, err := ic.Libpod.TmpDir()
	if err != nil {
		return err
	}
	pausePidPath, err := util.GetRootlessPauseProcessPidPathGivenDir(tmpDir)
	if err != nil {
		return fmt.Errorf("could not get pause process pid file path: %w", err)
	}

	became, ret, err := rootless.TryJoinPauseProcess(pausePidPath)
	if err != nil {
		return err
	}
	if became {
		os.Exit(ret)
	}
	if noMoveProcess {
		return nil
	}

	// if there is no pid file, try to join existing containers, and create a pause process.
	ctrs, err := ic.Libpod.GetRunningContainers()
	if err != nil {
		logrus.Error(err.Error())
		os.Exit(1)
	}

	paths := []string{}
	for _, ctr := range ctrs {
		paths = append(paths, ctr.ConfigNoCopy().ConmonPidFile)
	}

	if len(paths) > 0 {
		became, ret, err = rootless.TryJoinFromFilePaths(pausePidPath, true, paths)
	} else {
		became, ret, err = rootless.BecomeRootInUserNS(pausePidPath)
		if err == nil {
			utils.MovePauseProcessToScope(pausePidPath)
		}
	}
	if err != nil {
		logrus.Error(fmt.Errorf("invalid internal status, try resetting the pause process with %q: %w", os.Args[0]+" system migrate", err))
		os.Exit(1)
	}
	if became {
		os.Exit(ret)
	}
	return nil
}

// SystemPrune removes unused data from the system. Pruning pods, containers, networks, volumes and images.
func (ic *ContainerEngine) SystemPrune(ctx context.Context, options entities.SystemPruneOptions) (*entities.SystemPruneReport, error) {
	var systemPruneReport = new(entities.SystemPruneReport)

	if options.External {
		if options.All || options.Volume || len(options.Filters) > 0 {
			return nil, fmt.Errorf("system prune --external cannot be combined with other options")
		}
		err := ic.Libpod.GarbageCollect()
		if err != nil {
			return nil, err
		}
		return systemPruneReport, nil
	}

	filters := []string{}
	for k, v := range options.Filters {
		filters = append(filters, fmt.Sprintf("%s=%s", k, v[0]))
	}
	reclaimedSpace := (uint64)(0)
	found := true
	for found {
		found = false

		// TODO: Figure out cleaner way to handle all of the different PruneOptions
		// Remove all unused pods.
		podPruneReports, err := ic.prunePodHelper(ctx)
		if err != nil {
			return nil, err
		}
		if len(podPruneReports) > 0 {
			found = true
		}

		systemPruneReport.PodPruneReport = append(systemPruneReport.PodPruneReport, podPruneReports...)

		// Remove all unused containers.
		containerPruneOptions := entities.ContainerPruneOptions{}
		containerPruneOptions.Filters = (url.Values)(options.Filters)

		containerPruneReports, err := ic.ContainerPrune(ctx, containerPruneOptions)
		if err != nil {
			return nil, err
		}

		reclaimedSpace += reports.PruneReportsSize(containerPruneReports)
		systemPruneReport.ContainerPruneReports = append(systemPruneReport.ContainerPruneReports, containerPruneReports...)

		// Remove all unused images.
		imagePruneOptions := entities.ImagePruneOptions{
			All:    options.All,
			Filter: filters,
		}

		imageEngine := ImageEngine{Libpod: ic.Libpod}
		imagePruneReports, err := imageEngine.Prune(ctx, imagePruneOptions)
		if err != nil {
			return nil, err
		}
		if len(imagePruneReports) > 0 {
			found = true
		}

		reclaimedSpace += reports.PruneReportsSize(imagePruneReports)
		systemPruneReport.ImagePruneReports = append(systemPruneReport.ImagePruneReports, imagePruneReports...)

		// Remove all unused networks.
		networkPruneOptions := entities.NetworkPruneOptions{}
		networkPruneOptions.Filters = options.Filters

		networkPruneReports, err := ic.NetworkPrune(ctx, networkPruneOptions)
		if err != nil {
			return nil, err
		}
		if len(networkPruneReports) > 0 {
			found = true
		}

		// Networks reclaimedSpace are always '0'.
		systemPruneReport.NetworkPruneReports = append(systemPruneReport.NetworkPruneReports, networkPruneReports...)

		// Remove unused volume data.
		if options.Volume {
			volumePruneOptions := entities.VolumePruneOptions{}
			volumePruneOptions.Filters = (url.Values)(options.Filters)

			volumePruneReports, err := ic.VolumePrune(ctx, volumePruneOptions)
			if err != nil {
				return nil, err
			}
			if len(volumePruneReports) > 0 {
				found = true
			}

			reclaimedSpace += reports.PruneReportsSize(volumePruneReports)
			systemPruneReport.VolumePruneReports = append(systemPruneReport.VolumePruneReports, volumePruneReports...)
		}
	}

	systemPruneReport.ReclaimedSpace = reclaimedSpace
	return systemPruneReport, nil
}

func (ic *ContainerEngine) SystemDf(ctx context.Context, options entities.SystemDfOptions) (*entities.SystemDfReport, error) {
	var (
		dfImages = []*entities.SystemDfImageReport{}
	)

	imageStats, totalImageSize, err := ic.Libpod.LibimageRuntime().DiskUsage(ctx)
	if err != nil {
		return nil, err
	}

	for _, stat := range imageStats {
		report := entities.SystemDfImageReport{
			Repository: stat.Repository,
			Tag:        stat.Tag,
			ImageID:    stat.ID,
			Created:    stat.Created,
			Size:       stat.Size,
			SharedSize: stat.SharedSize,
			UniqueSize: stat.UniqueSize,
			Containers: stat.Containers,
		}
		dfImages = append(dfImages, &report)
	}

	// Get Containers and iterate them
	cons, err := ic.Libpod.GetAllContainers()
	if err != nil {
		return nil, err
	}
	dfContainers := make([]*entities.SystemDfContainerReport, 0, len(cons))
	for _, c := range cons {
		iid, _ := c.Image()
		state, err := c.State()
		if err != nil {
			return nil, fmt.Errorf("failed to get state of container %s: %w", c.ID(), err)
		}
		conSize, err := c.RootFsSize()
		if err != nil {
			if errors.Is(err, storage.ErrContainerUnknown) {
				logrus.Error(fmt.Errorf("failed to get root file system size of container %s: %w", c.ID(), err))
			} else {
				return nil, fmt.Errorf("failed to get root file system size of container %s: %w", c.ID(), err)
			}
		}
		rwsize, err := c.RWSize()
		if err != nil {
			if errors.Is(err, storage.ErrContainerUnknown) {
				logrus.Error(fmt.Errorf("failed to get read/write size of container %s: %w", c.ID(), err))
			} else {
				return nil, fmt.Errorf("failed to get read/write size of container %s: %w", c.ID(), err)
			}
		}
		report := entities.SystemDfContainerReport{
			ContainerID:  c.ID(),
			Image:        iid,
			Command:      c.Command(),
			LocalVolumes: len(c.UserVolumes()),
			RWSize:       rwsize,
			Size:         conSize,
			Created:      c.CreatedTime(),
			Status:       state.String(),
			Names:        c.Name(),
		}
		dfContainers = append(dfContainers, &report)
	}

	//	Get volumes and iterate them
	vols, err := ic.Libpod.GetAllVolumes()
	if err != nil {
		return nil, err
	}

	dfVolumes := make([]*entities.SystemDfVolumeReport, 0, len(vols))
	for _, v := range vols {
		var reclaimableSize uint64
		mountPoint, err := v.MountPoint()
		if err != nil {
			return nil, err
		}
		if mountPoint == "" {
			// We can't get any info on this volume, as it's not
			// mounted.
			// TODO: fix this.
			continue
		}
		volSize, err := util.SizeOfPath(mountPoint)
		if err != nil {
			return nil, err
		}
		inUse, err := v.VolumeInUse()
		if err != nil {
			return nil, err
		}
		if len(inUse) == 0 {
			reclaimableSize = volSize
		}
		report := entities.SystemDfVolumeReport{
			VolumeName:      v.Name(),
			Links:           len(inUse),
			Size:            int64(volSize),
			ReclaimableSize: int64(reclaimableSize),
		}
		dfVolumes = append(dfVolumes, &report)
	}

	return &entities.SystemDfReport{
		ImagesSize: totalImageSize,
		Images:     dfImages,
		Containers: dfContainers,
		Volumes:    dfVolumes,
	}, nil
}

func (se *SystemEngine) Reset(ctx context.Context) error {
	return nil
}

func (se *SystemEngine) Renumber(ctx context.Context, flags *pflag.FlagSet, config *entities.PodmanConfig) error {
	return nil
}

func (se SystemEngine) Migrate(ctx context.Context, flags *pflag.FlagSet, config *entities.PodmanConfig, options entities.SystemMigrateOptions) error {
	return nil
}

func (se SystemEngine) Shutdown(ctx context.Context) {
	if err := se.Libpod.Shutdown(false); err != nil {
		logrus.Error(err)
	}
}

func unshareEnv(graphroot, runroot string) []string {
	return append(os.Environ(), "_CONTAINERS_USERNS_CONFIGURED=done",
		fmt.Sprintf("CONTAINERS_GRAPHROOT=%s", graphroot),
		fmt.Sprintf("CONTAINERS_RUNROOT=%s", runroot))
}

func (ic *ContainerEngine) Unshare(ctx context.Context, args []string, options entities.SystemUnshareOptions) error {
	unshare := func() error {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Env = unshareEnv(ic.Libpod.StorageConfig().GraphRoot, ic.Libpod.StorageConfig().RunRoot)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	if options.RootlessNetNS {
		rootlessNetNS, err := ic.Libpod.GetRootlessNetNs(true)
		if err != nil {
			return err
		}
		// Make sure to unlock, unshare can run for a long time.
		rootlessNetNS.Lock.Unlock()
		// We do not want to clean up the netns after unshare.
		// The problem is that we cannot know if we need to clean up and
		// secondly unshare should allow user to set up the namespace with
		// special things, e.g. potentially macvlan or something like that.
		return rootlessNetNS.Do(unshare)
	}
	return unshare()
}

func (ic ContainerEngine) Version(ctx context.Context) (*entities.SystemVersionReport, error) {
	var report entities.SystemVersionReport
	v, err := define.GetVersion()
	if err != nil {
		return nil, err
	}
	report.Client = &v
	return &report, err
}
