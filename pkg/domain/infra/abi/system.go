package abi

import (
	"context"
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
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
)

func (ic *ContainerEngine) Info(ctx context.Context) (*define.Info, error) {
	info, err := ic.Libpod.Info()
	if err != nil {
		return nil, err
	}

	socketPath, err := util.SocketPath()
	if err != nil {
		return nil, err
	}
	rs := define.RemoteSocket{
		Path:   socketPath,
		Exists: false,
	}

	// Check if the socket exists
	if fi, err := os.Stat(socketPath); err == nil {
		if fi.Mode()&os.ModeSocket != 0 {
			rs.Exists = true
		}
	}
	// TODO
	// it was suggested future versions of this could perform
	// a ping on the socket for greater confidence the socket is
	// actually active.
	info.Host.RemoteSocket = &rs
	return info, err
}

func (ic *ContainerEngine) SetupRootless(_ context.Context, noMoveProcess bool) error {
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
			runsUnderSystemd := utils.RunsOnSystemd()
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
		return errors.Wrapf(err, "could not get pause process pid file path")
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
		paths = append(paths, ctr.Config().ConmonPidFile)
	}

	became, ret, err = rootless.TryJoinFromFilePaths(pausePidPath, true, paths)
	utils.MovePauseProcessToScope(pausePidPath)
	if err != nil {
		logrus.Error(errors.Wrapf(err, "invalid internal status, try resetting the pause process with %q", os.Args[0]+" system migrate"))
		os.Exit(1)
	}
	if became {
		os.Exit(ret)
	}
	return nil
}

// SystemPrune removes unused data from the system. Pruning pods, containers, volumes and images.
func (ic *ContainerEngine) SystemPrune(ctx context.Context, options entities.SystemPruneOptions) (*entities.SystemPruneReport, error) {
	var systemPruneReport = new(entities.SystemPruneReport)
	filters := []string{}
	for k, v := range options.Filters {
		filters = append(filters, fmt.Sprintf("%s=%s", k, v[0]))
	}
	reclaimedSpace := (uint64)(0)
	found := true
	for found {
		found = false
		podPruneReport, err := ic.prunePodHelper(ctx)
		if err != nil {
			return nil, err
		}
		if len(podPruneReport) > 0 {
			found = true
		}
		systemPruneReport.PodPruneReport = append(systemPruneReport.PodPruneReport, podPruneReport...)

		// TODO: Figure out cleaner way to handle all of the different PruneOptions
		containerPruneOptions := entities.ContainerPruneOptions{}
		containerPruneOptions.Filters = (url.Values)(options.Filters)

		containerPruneReports, err := ic.ContainerPrune(ctx, containerPruneOptions)
		if err != nil {
			return nil, err
		}
		reclaimedSpace = reclaimedSpace + reports.PruneReportsSize(containerPruneReports)
		systemPruneReport.ContainerPruneReports = append(systemPruneReport.ContainerPruneReports, containerPruneReports...)
		imagePruneOptions := entities.ImagePruneOptions{
			All:    options.All,
			Filter: filters,
		}
		imageEngine := ImageEngine{Libpod: ic.Libpod}
		imagePruneReports, err := imageEngine.Prune(ctx, imagePruneOptions)
		reclaimedSpace = reclaimedSpace + reports.PruneReportsSize(imagePruneReports)

		if err != nil {
			return nil, err
		}
		if len(imagePruneReports) > 0 {
			found = true
		}

		systemPruneReport.ImagePruneReports = append(systemPruneReport.ImagePruneReports, imagePruneReports...)
		if options.Volume {
			volumePruneOptions := entities.VolumePruneOptions{}
			volumePruneOptions.Filters = (url.Values)(options.Filters)
			volumePruneReport, err := ic.VolumePrune(ctx, volumePruneOptions)
			if err != nil {
				return nil, err
			}
			if len(volumePruneReport) > 0 {
				found = true
			}
			reclaimedSpace = reclaimedSpace + reports.PruneReportsSize(volumePruneReport)
			systemPruneReport.VolumePruneReports = append(systemPruneReport.VolumePruneReports, volumePruneReport...)
		}
	}
	systemPruneReport.ReclaimedSpace = reclaimedSpace
	return systemPruneReport, nil
}

func (ic *ContainerEngine) SystemDf(ctx context.Context, options entities.SystemDfOptions) (*entities.SystemDfReport, error) {
	var (
		dfImages = []*entities.SystemDfImageReport{}
	)

	imageStats, err := ic.Libpod.LibimageRuntime().DiskUsage(ctx)
	if err != nil {
		return nil, err
	}

	for _, stat := range imageStats {
		report := entities.SystemDfImageReport{
			Repository: stat.Repository,
			Tag:        stat.Tag,
			ImageID:    stat.ID,
			Created:    stat.Created,
			Size:       int64(stat.Size),
			SharedSize: int64(stat.SharedSize),
			UniqueSize: int64(stat.UniqueSize),
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
			return nil, errors.Wrapf(err, "Failed to get state of container %s", c.ID())
		}
		conSize, err := c.RootFsSize()
		if err != nil {
			if errors.Cause(err) == storage.ErrContainerUnknown {
				logrus.Error(errors.Wrapf(err, "Failed to get root file system size of container %s", c.ID()))
			} else {
				return nil, errors.Wrapf(err, "Failed to get root file system size of container %s", c.ID())
			}
		}
		rwsize, err := c.RWSize()
		if err != nil {
			if errors.Cause(err) == storage.ErrContainerUnknown {
				logrus.Error(errors.Wrapf(err, "Failed to get read/write size of container %s", c.ID()))
			} else {
				return nil, errors.Wrapf(err, "Failed to get read/write size of container %s", c.ID())
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

	running, err := ic.Libpod.GetRunningContainers()
	if err != nil {
		return nil, err
	}
	runningContainers := make([]string, 0, len(running))
	for _, c := range running {
		runningContainers = append(runningContainers, c.ID())
	}

	dfVolumes := make([]*entities.SystemDfVolumeReport, 0, len(vols))
	var reclaimableSize int64
	for _, v := range vols {
		var consInUse int
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
		volSize, err := sizeOfPath(mountPoint)
		if err != nil {
			return nil, err
		}
		inUse, err := v.VolumeInUse()
		if err != nil {
			return nil, err
		}
		if len(inUse) == 0 {
			reclaimableSize += volSize
		}
		for _, viu := range inUse {
			if util.StringInSlice(viu, runningContainers) {
				consInUse++
			}
		}
		report := entities.SystemDfVolumeReport{
			VolumeName:      v.Name(),
			Links:           consInUse,
			Size:            volSize,
			ReclaimableSize: reclaimableSize,
		}
		dfVolumes = append(dfVolumes, &report)
	}
	return &entities.SystemDfReport{
		Images:     dfImages,
		Containers: dfContainers,
		Volumes:    dfVolumes,
	}, nil
}

// sizeOfPath determines the file usage of a given path. it was called volumeSize in v1
// and now is made to be generic and take a path instead of a libpod volume
func sizeOfPath(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			size += info.Size()
		}
		return err
	})
	return size, err
}

func (se *SystemEngine) Reset(ctx context.Context) error {
	return se.Libpod.Reset(ctx)
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
		// We do not want to cleanup the netns after unshare.
		// The problem is that we cannot know if we need to cleanup and
		// secondly unshare should allow user to setup the namespace with
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
