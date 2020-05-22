package abi

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/containers/common/pkg/config"
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/cgroups"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/containers/libpod/pkg/util"
	"github.com/containers/libpod/utils"
	"github.com/docker/distribution/reference"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func (ic *ContainerEngine) Info(ctx context.Context) (*define.Info, error) {
	return ic.Libpod.Info()
}

func (ic *ContainerEngine) SetupRootless(_ context.Context, cmd *cobra.Command) error {
	// do it only after podman has already re-execed and running with uid==0.
	if os.Geteuid() == 0 {
		ownsCgroup, err := cgroups.UserOwnsCurrentSystemdCgroup()
		if err != nil {
			logrus.Warnf("Failed to detect the owner for the current cgroup: %v", err)
		}
		if !ownsCgroup {
			conf, err := ic.Config(context.Background())
			if err != nil {
				return err
			}
			unitName := fmt.Sprintf("podman-%d.scope", os.Getpid())
			if err := utils.RunUnderSystemdScope(os.Getpid(), "user.slice", unitName); err != nil {
				if conf.Engine.CgroupManager == config.SystemdCgroupsManager {
					logrus.Warnf("Failed to add podman to systemd sandbox cgroup: %v", err)
				} else {
					logrus.Debugf("Failed to add podman to systemd sandbox cgroup: %v", err)
				}
			}
		}
		return nil
	}

	pausePidPath, err := util.GetRootlessPauseProcessPidPath()
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
	if err := movePauseProcessToScope(); err != nil {
		conf, err := ic.Config(context.Background())
		if err != nil {
			return err
		}
		if conf.Engine.CgroupManager == config.SystemdCgroupsManager {
			logrus.Warnf("Failed to add pause process to systemd sandbox cgroup: %v", err)
		} else {
			logrus.Debugf("Failed to add pause process to systemd sandbox cgroup: %v", err)
		}
	}
	if err != nil {
		logrus.Error(err)
		os.Exit(1)
	}
	if became {
		os.Exit(ret)
	}
	return nil
}

func movePauseProcessToScope() error {
	pausePidPath, err := util.GetRootlessPauseProcessPidPath()
	if err != nil {
		return errors.Wrapf(err, "could not get pause process pid file path")
	}

	data, err := ioutil.ReadFile(pausePidPath)
	if err != nil {
		return errors.Wrapf(err, "cannot read pause pid file")
	}
	pid, err := strconv.ParseUint(string(data), 10, 0)
	if err != nil {
		return errors.Wrapf(err, "cannot parse pid file %s", pausePidPath)
	}

	return utils.RunUnderSystemdScope(int(pid), "user.slice", "podman-pause.scope")
}

func setRLimits() error { // nolint:deadcode,unused
	rlimits := new(syscall.Rlimit)
	rlimits.Cur = 1048576
	rlimits.Max = 1048576
	if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, rlimits); err != nil {
		if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, rlimits); err != nil {
			return errors.Wrapf(err, "error getting rlimits")
		}
		rlimits.Cur = rlimits.Max
		if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, rlimits); err != nil {
			return errors.Wrapf(err, "error setting new rlimits")
		}
	}
	return nil
}

func setUMask() { // nolint:deadcode,unused
	// Be sure we can create directories with 0755 mode.
	syscall.Umask(0022)
}

// checkInput can be used to verify any of the globalopt values
func checkInput() error { // nolint:deadcode,unused
	return nil
}

// SystemPrune removes unsed data from the system. Pruning pods, containers, volumes and images.
func (ic *ContainerEngine) SystemPrune(ctx context.Context, options entities.SystemPruneOptions) (*entities.SystemPruneReport, error) {
	var systemPruneReport = new(entities.SystemPruneReport)
	podPruneReport, err := ic.prunePodHelper(ctx)
	if err != nil {
		return nil, err
	}
	systemPruneReport.PodPruneReport = podPruneReport

	containerPruneReport, err := ic.pruneContainersHelper(ctx, nil)
	if err != nil {
		return nil, err
	}
	systemPruneReport.ContainerPruneReport = containerPruneReport

	results, err := ic.Libpod.ImageRuntime().PruneImages(ctx, options.All, nil)
	if err != nil {
		return nil, err
	}
	report := entities.ImagePruneReport{
		Report: entities.Report{
			Id:  results,
			Err: nil,
		},
	}

	systemPruneReport.ImagePruneReport = &report

	if options.Volume {
		volumePruneReport, err := ic.pruneVolumesHelper(ctx)
		if err != nil {
			return nil, err
		}
		systemPruneReport.VolumePruneReport = volumePruneReport
	}
	return systemPruneReport, nil
}

func (ic *ContainerEngine) SystemDf(ctx context.Context, options entities.SystemDfOptions) (*entities.SystemDfReport, error) {
	var (
		dfImages          []*entities.SystemDfImageReport
		dfContainers      []*entities.SystemDfContainerReport
		dfVolumes         []*entities.SystemDfVolumeReport
		runningContainers []string
	)

	// Get Images and iterate them
	imgs, err := ic.Libpod.ImageRuntime().GetImages()
	if err != nil {
		return nil, err
	}
	for _, i := range imgs {
		var sharedSize uint64
		cons, err := i.Containers()
		if err != nil {
			return nil, err
		}
		imageSize, err := i.Size(ctx)
		if err != nil {
			return nil, err
		}
		uniqueSize := *imageSize

		parent, err := i.GetParent(ctx)
		if err != nil {
			return nil, err
		}
		if parent != nil {
			parentSize, err := parent.Size(ctx)
			if err != nil {
				return nil, err
			}
			uniqueSize = *parentSize - *imageSize
			sharedSize = *imageSize - uniqueSize
		}
		var name, repository, tag string
		for _, n := range i.Names() {
			if len(n) > 0 {
				name = n
				break
			}
		}

		named, err := reference.ParseNormalizedNamed(name)
		if err != nil {
			return nil, err
		}
		repository = named.Name()
		if tagged, isTagged := named.(reference.NamedTagged); isTagged {
			tag = tagged.Tag()
		}

		report := entities.SystemDfImageReport{
			Repository: repository,
			Tag:        tag,
			ImageID:    i.ID(),
			Created:    i.Created(),
			Size:       int64(*imageSize),
			SharedSize: int64(sharedSize),
			UniqueSize: int64(uniqueSize),
			Containers: len(cons),
		}
		dfImages = append(dfImages, &report)
	}

	//	GetContainers and iterate them
	cons, err := ic.Libpod.GetAllContainers()
	if err != nil {
		return nil, err
	}
	for _, c := range cons {
		iid, _ := c.Image()
		conSize, err := c.RootFsSize()
		if err != nil {
			return nil, err
		}
		state, err := c.State()
		if err != nil {
			return nil, err
		}
		rwsize, err := c.RWSize()
		if err != nil {
			return nil, err
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
	for _, c := range running {
		runningContainers = append(runningContainers, c.ID())
	}

	for _, v := range vols {
		var consInUse int
		volSize, err := sizeOfPath(v.MountPoint())
		if err != nil {
			return nil, err
		}
		inUse, err := v.VolumesInUse()
		if err != nil {
			return nil, err
		}
		for _, viu := range inUse {
			if util.StringInSlice(viu, runningContainers) {
				consInUse += 1
			}
		}
		report := entities.SystemDfVolumeReport{
			VolumeName: v.Name(),
			Links:      consInUse,
			Size:       volSize,
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

func (s SystemEngine) Migrate(ctx context.Context, flags *pflag.FlagSet, config *entities.PodmanConfig, options entities.SystemMigrateOptions) error {
	return nil
}

func (s SystemEngine) Shutdown(ctx context.Context) {
	if err := s.Libpod.Shutdown(false); err != nil {
		logrus.Error(err)
	}
}

func unshareEnv(graphroot, runroot string) []string {
	return append(os.Environ(), "_CONTAINERS_USERNS_CONFIGURED=done",
		fmt.Sprintf("CONTAINERS_GRAPHROOT=%s", graphroot),
		fmt.Sprintf("CONTAINERS_RUNROOT=%s", runroot))
}

func (ic *ContainerEngine) Unshare(ctx context.Context, args []string) error {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Env = unshareEnv(ic.Libpod.StorageConfig().GraphRoot, ic.Libpod.StorageConfig().RunRoot)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
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
