package abi

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"sync"
	"time"

	"github.com/containers/buildah"
	"github.com/containers/common/pkg/cgroups"
	"github.com/containers/common/pkg/config"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/podman/v4/libpod"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/libpod/logs"
	"github.com/containers/podman/v4/pkg/api/handlers"
	"github.com/containers/podman/v4/pkg/checkpoint"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/domain/entities/reports"
	dfilters "github.com/containers/podman/v4/pkg/domain/filters"
	"github.com/containers/podman/v4/pkg/domain/infra/abi/terminal"
	"github.com/containers/podman/v4/pkg/errorhandling"
	parallelctr "github.com/containers/podman/v4/pkg/parallel/ctr"
	"github.com/containers/podman/v4/pkg/ps"
	"github.com/containers/podman/v4/pkg/rootless"
	"github.com/containers/podman/v4/pkg/signal"
	"github.com/containers/podman/v4/pkg/specgen"
	"github.com/containers/podman/v4/pkg/specgen/generate"
	"github.com/containers/podman/v4/pkg/specgenutil"
	"github.com/containers/podman/v4/pkg/util"
	"github.com/containers/storage"
	"github.com/sirupsen/logrus"
)

type getContainersOptions struct {
	all     bool
	isPod   bool
	ignore  bool
	latest  bool
	running bool
	filters map[string][]string
	names   []string
}

type containerWrapper struct {
	*libpod.Container
	rawInput     string
	doesNotExist bool
}

func getContainers(runtime *libpod.Runtime, options getContainersOptions) ([]containerWrapper, error) {
	var libpodContainers []*libpod.Container

	switch {
	case len(options.filters) > 0:
		filterFuncs := make([]libpod.ContainerFilter, 0, len(options.filters))
		for k, v := range options.filters {
			generatedFunc, err := dfilters.GenerateContainerFilterFuncs(k, v, runtime)
			if err != nil {
				return nil, err
			}
			filterFuncs = append(filterFuncs, generatedFunc)
		}
		ctrs, err := runtime.GetContainers(false, filterFuncs...)
		if err != nil {
			return nil, err
		}
		libpodContainers = ctrs
		if len(options.names) > 0 {
			var filteredCtrs []*libpod.Container
			for _, candidate := range ctrs {
				for _, name := range options.names {
					if candidate.ID() == name || candidate.Name() == name {
						filteredCtrs = append(filteredCtrs, candidate)
					}
				}
				libpodContainers = filteredCtrs
			}
		}
	case options.running:
		// Process `running` before `all`. podman-restart allows both
		// but will narrow it down to `running`.
		containers, err := runtime.GetRunningContainers()
		if err != nil {
			return nil, err
		}
		libpodContainers = containers
	case options.all:
		containers, err := runtime.GetAllContainers()
		if err != nil {
			return nil, err
		}
		libpodContainers = containers
	case options.latest:
		if options.isPod {
			pod, err := runtime.GetLatestPod()
			if err != nil {
				return nil, err
			}
			podCtrs, err := pod.AllContainers()
			if err != nil {
				return nil, err
			}
			libpodContainers = podCtrs
		} else {
			ctr, err := runtime.GetLatestContainer()
			if err != nil {
				return nil, err
			}
			libpodContainers = append(libpodContainers, ctr)
		}
	default:
		containers := make([]containerWrapper, 0, len(options.names))
		for _, n := range options.names {
			ctr, err := runtime.LookupContainer(n)
			if err != nil {
				if options.ignore && errors.Is(err, define.ErrNoSuchCtr) {
					containers = append(containers, containerWrapper{rawInput: n, doesNotExist: true})
					continue
				}
				return nil, err
			}
			containers = append(containers, containerWrapper{Container: ctr, rawInput: n})
		}
		return containers, nil
	}

	// If no names or IDs are specified, we can return the result as is.
	// Otherwise, we need to do some further lookups.
	if len(options.names) == 0 || (len(options.names) == 1 && options.names[0] == "") {
		containers := make([]containerWrapper, len(libpodContainers))
		for i, c := range libpodContainers {
			containers[i] = containerWrapper{Container: libpodContainers[i], rawInput: c.ID()}
		}
		return containers, nil
	}

	containers := []containerWrapper{}
	for _, n := range options.names {
		c, err := runtime.LookupContainer(n)
		if err != nil {
			return nil, err
		}
		for i, lc := range libpodContainers {
			if c.ID() == lc.ID() {
				containers = append(containers, containerWrapper{Container: libpodContainers[i], rawInput: n})
				break
			}
		}
	}

	return containers, nil
}

// ContainerExists returns whether the container exists in container storage
func (ic *ContainerEngine) ContainerExists(ctx context.Context, nameOrID string, options entities.ContainerExistsOptions) (*entities.BoolReport, error) {
	_, err := ic.Libpod.LookupContainer(nameOrID)
	if err != nil {
		if !errors.Is(err, define.ErrNoSuchCtr) {
			return nil, err
		}
		if options.External {
			// Check if container exists in storage
			if _, storageErr := ic.Libpod.StorageContainer(nameOrID); storageErr == nil {
				err = nil
			}
		}
	}
	return &entities.BoolReport{Value: err == nil}, nil
}

func (ic *ContainerEngine) ContainerWait(ctx context.Context, namesOrIds []string, options entities.WaitOptions) ([]entities.WaitReport, error) {
	responses := make([]entities.WaitReport, 0, len(namesOrIds))
	containers, err := getContainers(ic.Libpod, getContainersOptions{latest: options.Latest, ignore: options.Ignore, names: namesOrIds})
	if err != nil {
		return nil, err
	}
	for _, c := range containers {
		if c.doesNotExist { // Only set when `options.Ignore == true`
			responses = append(responses, entities.WaitReport{ExitCode: -1})
			continue
		}

		response := entities.WaitReport{}
		var conditions []string
		if len(options.Conditions) == 0 {
			conditions = []string{define.ContainerStateStopped.String(), define.ContainerStateExited.String()}
		} else {
			conditions = options.Conditions
		}
		exitCode, err := c.WaitForConditionWithInterval(ctx, options.Interval, conditions...)
		if err != nil {
			response.Error = err
		} else {
			response.ExitCode = exitCode
		}
		responses = append(responses, response)
	}
	return responses, nil
}

func (ic *ContainerEngine) ContainerPause(ctx context.Context, namesOrIds []string, options entities.PauseUnPauseOptions) ([]*entities.PauseUnpauseReport, error) {
	containers, err := getContainers(ic.Libpod, getContainersOptions{all: options.All, latest: options.Latest, names: namesOrIds, filters: options.Filters})
	if err != nil {
		return nil, err
	}
	reports := make([]*entities.PauseUnpauseReport, 0, len(containers))
	for _, c := range containers {
		err := c.Pause()
		if err != nil && options.All && errors.Is(err, define.ErrCtrStateInvalid) {
			logrus.Debugf("Container %s is not running", c.ID())
			continue
		}
		reports = append(reports, &entities.PauseUnpauseReport{
			Id:       c.ID(),
			Err:      err,
			RawInput: c.rawInput,
		})
	}
	return reports, nil
}

func (ic *ContainerEngine) ContainerUnpause(ctx context.Context, namesOrIds []string, options entities.PauseUnPauseOptions) ([]*entities.PauseUnpauseReport, error) {
	containers, err := getContainers(ic.Libpod, getContainersOptions{all: options.All, latest: options.Latest, names: namesOrIds, filters: options.Filters})
	if err != nil {
		return nil, err
	}
	reports := make([]*entities.PauseUnpauseReport, 0, len(containers))
	for _, c := range containers {
		err := c.Unpause()
		if err != nil && options.All && errors.Is(err, define.ErrCtrStateInvalid) {
			logrus.Debugf("Container %s is not paused", c.ID())
			continue
		}
		reports = append(reports, &entities.PauseUnpauseReport{
			Id:       c.ID(),
			Err:      err,
			RawInput: c.rawInput,
		})
	}
	return reports, nil
}
func (ic *ContainerEngine) ContainerStop(ctx context.Context, namesOrIds []string, options entities.StopOptions) ([]*entities.StopReport, error) {
	containers, err := getContainers(ic.Libpod,
		getContainersOptions{
			all:     options.All,
			latest:  options.Latest,
			names:   namesOrIds,
			filters: options.Filters,
			ignore:  options.Ignore,
		})
	if err != nil {
		return nil, err
	}

	idToRawInput := make(map[string]string, len(containers))
	libpodContainers := make([]*libpod.Container, 0, len(containers))
	for i, c := range containers {
		if c.doesNotExist {
			continue
		}
		idToRawInput[c.ID()] = c.rawInput
		libpodContainers = append(libpodContainers, containers[i].Container)
	}

	errMap, err := parallelctr.ContainerOp(ctx, libpodContainers, func(c *libpod.Container) error {
		var err error
		if options.Timeout != nil {
			err = c.StopWithTimeout(*options.Timeout)
		} else {
			err = c.Stop()
		}
		if err != nil {
			switch {
			case errors.Is(err, define.ErrCtrStopped):
				logrus.Debugf("Container %s is already stopped", c.ID())
			case options.All && errors.Is(err, define.ErrCtrStateInvalid):
				logrus.Debugf("Container %s is not running, could not stop", c.ID())
			// container never created in OCI runtime
			// docker parity: do nothing just return container id
			case errors.Is(err, define.ErrCtrStateInvalid):
				logrus.Debugf("Container %s is either not created on runtime or is in an invalid state", c.ID())
			default:
				return err
			}
		}
		err = c.Cleanup(ctx)
		if err != nil {
			// Issue #7384 and #11384: If the container is configured for
			// auto-removal, it might already have been removed at this point.
			// We still need to clean up since we do not know if the other cleanup process is successful
			if c.AutoRemove() && (errors.Is(err, define.ErrNoSuchCtr) || errors.Is(err, define.ErrCtrRemoved)) {
				return nil
			}
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	reports := make([]*entities.StopReport, 0, len(errMap))
	for ctr, err := range errMap {
		report := new(entities.StopReport)
		report.Id = ctr.ID()
		report.RawInput = idToRawInput[ctr.ID()]
		report.Err = err
		reports = append(reports, report)
	}
	return reports, nil
}

func (ic *ContainerEngine) ContainerPrune(ctx context.Context, options entities.ContainerPruneOptions) ([]*reports.PruneReport, error) {
	filterFuncs := make([]libpod.ContainerFilter, 0, len(options.Filters))
	for k, v := range options.Filters {
		generatedFunc, err := dfilters.GeneratePruneContainerFilterFuncs(k, v, ic.Libpod)
		if err != nil {
			return nil, err
		}

		filterFuncs = append(filterFuncs, generatedFunc)
	}
	return ic.Libpod.PruneContainers(filterFuncs)
}

func (ic *ContainerEngine) ContainerKill(ctx context.Context, namesOrIds []string, options entities.KillOptions) ([]*entities.KillReport, error) {
	sig, err := signal.ParseSignalNameOrNumber(options.Signal)
	if err != nil {
		return nil, err
	}
	containers, err := getContainers(ic.Libpod, getContainersOptions{all: options.All, latest: options.Latest, names: namesOrIds})
	if err != nil {
		return nil, err
	}

	reports := make([]*entities.KillReport, 0, len(containers))
	for _, con := range containers {
		err := con.Kill(uint(sig))
		if options.All && errors.Is(err, define.ErrCtrStateInvalid) {
			logrus.Debugf("Container %s is not running", con.ID())
			continue
		}
		reports = append(reports, &entities.KillReport{
			Id:       con.ID(),
			Err:      err,
			RawInput: con.rawInput,
		})
	}
	return reports, nil
}

func (ic *ContainerEngine) ContainerRestart(ctx context.Context, namesOrIds []string, options entities.RestartOptions) ([]*entities.RestartReport, error) {
	containers, err := getContainers(ic.Libpod, getContainersOptions{
		all:     options.All,
		filters: options.Filters,
		latest:  options.Latest,
		running: options.Running,
		names:   namesOrIds,
	})
	if err != nil {
		return nil, err
	}

	reports := make([]*entities.RestartReport, 0, len(containers))
	for _, c := range containers {
		timeout := c.StopTimeout()
		if options.Timeout != nil {
			timeout = *options.Timeout
		}
		reports = append(reports, &entities.RestartReport{
			Id:       c.ID(),
			Err:      c.RestartWithTimeout(ctx, timeout),
			RawInput: c.rawInput,
		})
	}
	return reports, nil
}

//nolint:unparam
func (ic *ContainerEngine) removeContainer(ctx context.Context, ctr *libpod.Container, options entities.RmOptions) (map[string]error, map[string]error, error) {
	var err error
	ctrs := make(map[string]error)
	pods := make(map[string]error)
	if options.All || options.Depend {
		ctrs, pods, err = ic.Libpod.RemoveContainerAndDependencies(ctx, ctr, options.Force, options.Volumes, options.Timeout)
	} else {
		err = ic.Libpod.RemoveContainer(ctx, ctr, options.Force, options.Volumes, options.Timeout)
	}
	ctrs[ctr.ID()] = err
	if err == nil {
		return ctrs, pods, nil
	}
	logrus.Debugf("Failed to remove container %s: %s", ctr.ID(), err.Error())
	if errors.Is(err, define.ErrNoSuchCtr) || errors.Is(err, define.ErrCtrRemoved) {
		return ctrs, pods, nil
	}
	return ctrs, pods, err
}

func (ic *ContainerEngine) ContainerRm(ctx context.Context, namesOrIds []string, options entities.RmOptions) ([]*reports.RmReport, error) {
	rmReports := []*reports.RmReport{}

	containers, err := getContainers(ic.Libpod, getContainersOptions{
		all:     options.All,
		latest:  options.Latest,
		filters: options.Filters,
		names:   namesOrIds,
		ignore:  true, // Force ignore as `podman rm` also handles external containers
	})
	if err != nil {
		return nil, err
	}

	libpodContainers := make([]*libpod.Container, 0, len(containers))
	idToRawInput := make(map[string]string, len(containers))
	for i, ctr := range containers {
		if ctr.doesNotExist {
			// If the container does not exist in Podman's database, it may
			// be an external one. Hence, try removing the external
			// "storage" container.
			if err := ic.Libpod.RemoveStorageContainer(ctr.rawInput, options.Force); err != nil {
				if options.Ignore && (errors.Is(err, define.ErrNoSuchCtr) || errors.Is(err, define.ErrCtrExists)) {
					continue
				}
				return nil, err
			}
			rmReports = append(rmReports, &reports.RmReport{RawInput: ctr.rawInput})
		} else {
			// If the container exists in the Podman database, we
			// can remove it correctly below.
			libpodContainers = append(libpodContainers, containers[i].Container)
			idToRawInput[ctr.ID()] = ctr.rawInput
		}
	}

	ctrsMap := make(map[string]error)
	mapMutex := sync.Mutex{}

	errMap, err := parallelctr.ContainerOp(ctx, libpodContainers, func(c *libpod.Container) error {
		mapMutex.Lock()
		if _, ok := ctrsMap[c.ID()]; ok {
			return nil
		}
		mapMutex.Unlock()

		// TODO: We should report removed pods back somehow.
		ctrs, _, err := ic.removeContainer(ctx, c, options)

		mapMutex.Lock()
		defer mapMutex.Unlock()
		for ctr, err := range ctrs {
			ctrsMap[ctr] = err
		}

		return err
	})
	if err != nil {
		return nil, err
	}

	for ctr, err := range errMap {
		if _, ok := ctrsMap[ctr.ID()]; ok {
			logrus.Debugf("Multiple results for container %s - attempted multiple removals?", ctr.ID())
		}
		ctrsMap[ctr.ID()] = err
	}

	for ctr, err := range ctrsMap {
		report := new(reports.RmReport)
		report.Id = ctr
		if !(errors.Is(err, define.ErrNoSuchCtr) || errors.Is(err, define.ErrCtrRemoved)) {
			report.Err = err
		}
		report.RawInput = idToRawInput[ctr]
		rmReports = append(rmReports, report)
	}

	return rmReports, nil
}

func (ic *ContainerEngine) ContainerInspect(ctx context.Context, namesOrIds []string, options entities.InspectOptions) ([]*entities.ContainerInspectReport, []error, error) {
	if options.Latest {
		ctr, err := ic.Libpod.GetLatestContainer()
		if err != nil {
			if errors.Is(err, define.ErrNoSuchCtr) {
				return nil, []error{fmt.Errorf("no containers to inspect: %w", err)}, nil
			}
			return nil, nil, err
		}

		inspect, err := ctr.Inspect(options.Size)
		if err != nil {
			return nil, nil, err
		}

		return []*entities.ContainerInspectReport{
			{
				InspectContainerData: inspect,
			},
		}, nil, nil
	}
	var (
		reports = make([]*entities.ContainerInspectReport, 0, len(namesOrIds))
		errs    = []error{}
	)
	for _, name := range namesOrIds {
		ctr, err := ic.Libpod.LookupContainer(name)
		if err != nil {
			// ErrNoSuchCtr is non-fatal, other errors will be
			// treated as fatal.
			if errors.Is(err, define.ErrNoSuchCtr) {
				errs = append(errs, fmt.Errorf("no such container %s", name))
				continue
			}
			return nil, nil, err
		}

		inspect, err := ctr.Inspect(options.Size)
		if err != nil {
			// ErrNoSuchCtr is non-fatal, other errors will be
			// treated as fatal.
			if errors.Is(err, define.ErrNoSuchCtr) {
				errs = append(errs, fmt.Errorf("no such container %s", name))
				continue
			}
			return nil, nil, err
		}

		reports = append(reports, &entities.ContainerInspectReport{InspectContainerData: inspect})
	}
	return reports, errs, nil
}

func (ic *ContainerEngine) ContainerTop(ctx context.Context, options entities.TopOptions) (*entities.StringSliceReport, error) {
	var (
		container *libpod.Container
		err       error
	)

	// Look up the container.
	if options.Latest {
		container, err = ic.Libpod.GetLatestContainer()
	} else {
		container, err = ic.Libpod.LookupContainer(options.NameOrID)
	}
	if err != nil {
		return nil, fmt.Errorf("unable to look up requested container: %w", err)
	}

	// Run Top.
	report := &entities.StringSliceReport{}
	report.Value, err = container.Top(options.Descriptors)
	return report, err
}

func (ic *ContainerEngine) ContainerCommit(ctx context.Context, nameOrID string, options entities.CommitOptions) (*entities.CommitReport, error) {
	var (
		mimeType string
	)
	ctr, err := ic.Libpod.LookupContainer(nameOrID)
	if err != nil {
		return nil, err
	}
	rtc, err := ic.Libpod.GetConfig()
	if err != nil {
		return nil, err
	}
	switch options.Format {
	case "oci":
		mimeType = buildah.OCIv1ImageManifest
		if len(options.Message) > 0 {
			return nil, fmt.Errorf("messages are only compatible with the docker image format (-f docker)")
		}
	case "docker":
		mimeType = manifest.DockerV2Schema2MediaType
	default:
		return nil, fmt.Errorf("unrecognized image format %q", options.Format)
	}

	sc := ic.Libpod.SystemContext()
	var changes []string
	if len(options.Changes) > 0 {
		changes = handlers.DecodeChanges(options.Changes)
	}
	var overrideConfig *manifest.Schema2Config
	if len(options.Config) > 0 {
		if overrideConfig, err = DecodeOverrideConfig(bytes.NewReader(options.Config)); err != nil {
			return nil, err
		}
	}
	coptions := buildah.CommitOptions{
		SignaturePolicyPath:   rtc.Engine.SignaturePolicyPath,
		ReportWriter:          options.Writer,
		SystemContext:         sc,
		PreferredManifestType: mimeType,
		OverrideConfig:        overrideConfig,
	}
	opts := libpod.ContainerCommitOptions{
		CommitOptions:  coptions,
		Pause:          options.Pause,
		IncludeVolumes: options.IncludeVolumes,
		Message:        options.Message,
		Changes:        changes,
		Author:         options.Author,
		Squash:         options.Squash,
	}
	newImage, err := ctr.Commit(ctx, options.ImageName, opts)
	if err != nil {
		return nil, err
	}
	return &entities.CommitReport{Id: newImage.ID()}, nil
}

func (ic *ContainerEngine) ContainerExport(ctx context.Context, nameOrID string, options entities.ContainerExportOptions) error {
	ctr, err := ic.Libpod.LookupContainer(nameOrID)
	if err != nil {
		return err
	}
	return ctr.Export(options.Output)
}

func (ic *ContainerEngine) ContainerCheckpoint(ctx context.Context, namesOrIds []string, options entities.CheckpointOptions) ([]*entities.CheckpointReport, error) {
	checkOpts := libpod.ContainerCheckpointOptions{
		Keep:           options.Keep,
		TCPEstablished: options.TCPEstablished,
		TargetFile:     options.Export,
		IgnoreRootfs:   options.IgnoreRootFS,
		IgnoreVolumes:  options.IgnoreVolumes,
		KeepRunning:    options.LeaveRunning,
		PreCheckPoint:  options.PreCheckPoint,
		WithPrevious:   options.WithPrevious,
		Compression:    options.Compression,
		PrintStats:     options.PrintStats,
		FileLocks:      options.FileLocks,
		CreateImage:    options.CreateImage,
	}
	// NOTE: all maps to running
	containers, err := getContainers(ic.Libpod, getContainersOptions{running: options.All, latest: options.Latest, names: namesOrIds})
	if err != nil {
		return nil, err
	}

	reports := make([]*entities.CheckpointReport, 0, len(containers))
	for _, c := range containers {
		criuStatistics, runtimeCheckpointDuration, err := c.Checkpoint(ctx, checkOpts)
		reports = append(reports, &entities.CheckpointReport{
			Err:             err,
			Id:              c.ID(),
			RawInput:        c.rawInput,
			RuntimeDuration: runtimeCheckpointDuration,
			CRIUStatistics:  criuStatistics,
		})
	}
	return reports, nil
}

func (ic *ContainerEngine) ContainerRestore(ctx context.Context, namesOrIds []string, options entities.RestoreOptions) ([]*entities.RestoreReport, error) {
	var (
		ctrs                        []*libpod.Container
		checkpointImageImportErrors []error
		err                         error
	)

	restoreOptions := libpod.ContainerCheckpointOptions{
		Keep:            options.Keep,
		TCPEstablished:  options.TCPEstablished,
		TargetFile:      options.Import,
		Name:            options.Name,
		IgnoreRootfs:    options.IgnoreRootFS,
		IgnoreVolumes:   options.IgnoreVolumes,
		IgnoreStaticIP:  options.IgnoreStaticIP,
		IgnoreStaticMAC: options.IgnoreStaticMAC,
		ImportPrevious:  options.ImportPrevious,
		Pod:             options.Pod,
		PrintStats:      options.PrintStats,
		FileLocks:       options.FileLocks,
	}

	filterFuncs := []libpod.ContainerFilter{
		func(c *libpod.Container) bool {
			state, _ := c.State()
			return state == define.ContainerStateExited
		},
	}

	idToRawInput := map[string]string{}
	switch {
	case options.Import != "":
		ctrs, err = checkpoint.CRImportCheckpointTar(ctx, ic.Libpod, options)
	case options.All:
		ctrs, err = ic.Libpod.GetContainers(false, filterFuncs...)
	case options.Latest:
		containers, err := getContainers(ic.Libpod, getContainersOptions{latest: options.Latest, names: namesOrIds})
		if err != nil {
			return nil, err
		}
		ctrs = make([]*libpod.Container, 0, len(containers))
		for i := range containers {
			ctrs = append(ctrs, containers[i].Container)
		}
	default:
		for _, nameOrID := range namesOrIds {
			logrus.Debugf("look up container: %q", nameOrID)
			c, err := ic.Libpod.LookupContainer(nameOrID)
			if err == nil {
				ctrs = append(ctrs, c)
				idToRawInput[c.ID()] = nameOrID
			} else {
				// If container was not found, check if this is a checkpoint image
				logrus.Debugf("look up image: %q", nameOrID)
				img, _, err := ic.Libpod.LibimageRuntime().LookupImage(nameOrID, nil)
				if err != nil {
					return nil, fmt.Errorf("no such container or image: %s", nameOrID)
				}
				restoreOptions.CheckpointImageID = img.ID()
				mountPoint, err := img.Mount(ctx, nil, "")
				defer func() {
					if err := img.Unmount(true); err != nil {
						logrus.Errorf("Failed to unmount image: %v", err)
					}
				}()
				if err != nil {
					return nil, err
				}
				importedCtrs, err := checkpoint.CRImportCheckpoint(ctx, ic.Libpod, options, mountPoint)
				if err != nil {
					// CRImportCheckpoint is expected to import exactly one container from checkpoint image
					checkpointImageImportErrors = append(
						checkpointImageImportErrors,
						fmt.Errorf("unable to import checkpoint from image: %q: %v", nameOrID, err),
					)
				} else {
					ctrs = append(ctrs, importedCtrs[0])
				}
			}
		}
	}
	if err != nil {
		return nil, err
	}

	reports := make([]*entities.RestoreReport, 0, len(ctrs))
	for _, c := range ctrs {
		criuStatistics, runtimeRestoreDuration, err := c.Restore(ctx, restoreOptions)
		reports = append(reports, &entities.RestoreReport{
			Err:             err,
			Id:              c.ID(),
			RawInput:        idToRawInput[c.ID()],
			RuntimeDuration: runtimeRestoreDuration,
			CRIUStatistics:  criuStatistics,
		})
	}

	for _, importErr := range checkpointImageImportErrors {
		reports = append(reports, &entities.RestoreReport{
			Err: importErr,
		})
	}

	return reports, nil
}

func (ic *ContainerEngine) ContainerCreate(ctx context.Context, s *specgen.SpecGenerator) (*entities.ContainerCreateReport, error) {
	warn, err := generate.CompleteSpec(ctx, ic.Libpod, s)
	if err != nil {
		return nil, err
	}
	// Print warnings
	for _, w := range warn {
		fmt.Fprintf(os.Stderr, "%s\n", w)
	}
	rtSpec, spec, opts, err := generate.MakeContainer(context.Background(), ic.Libpod, s, false, nil)
	if err != nil {
		return nil, err
	}
	ctr, err := generate.ExecuteCreate(ctx, ic.Libpod, rtSpec, spec, false, opts...)
	if err != nil {
		return nil, err
	}
	return &entities.ContainerCreateReport{Id: ctr.ID()}, nil
}

func (ic *ContainerEngine) ContainerAttach(ctx context.Context, nameOrID string, options entities.AttachOptions) error {
	containers, err := getContainers(ic.Libpod, getContainersOptions{latest: options.Latest, names: []string{nameOrID}})
	if err != nil {
		return err
	}
	if len(containers) != 1 {
		return fmt.Errorf("%w: expected to find exactly one container but got %d", define.ErrInternal, len(containers))
	}

	ctr := containers[0]
	conState, err := ctr.State()
	if err != nil {
		return fmt.Errorf("unable to determine state of %s: %w", ctr.ID(), err)
	}
	if conState != define.ContainerStateRunning {
		return fmt.Errorf("you can only attach to running containers")
	}

	// If the container is in a pod, also set to recursively start dependencies
	err = terminal.StartAttachCtr(ctx, ctr.Container, options.Stdout, options.Stderr, options.Stdin, options.DetachKeys, options.SigProxy, false)
	if err != nil && !errors.Is(err, define.ErrDetach) {
		return fmt.Errorf("attaching to container %s: %w", ctr.ID(), err)
	}
	os.Stdout.WriteString("\n")
	return nil
}

func makeExecConfig(options entities.ExecOptions, rt *libpod.Runtime) (*libpod.ExecConfig, error) {
	execConfig := new(libpod.ExecConfig)
	execConfig.Command = options.Cmd
	execConfig.Terminal = options.Tty
	execConfig.Privileged = options.Privileged
	execConfig.Environment = options.Envs
	execConfig.User = options.User
	execConfig.WorkDir = options.WorkDir
	execConfig.DetachKeys = &options.DetachKeys
	execConfig.PreserveFDs = options.PreserveFDs
	execConfig.AttachStdin = options.Interactive

	// Make an exit command
	storageConfig := rt.StorageConfig()
	runtimeConfig, err := rt.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("retrieving Libpod configuration to build exec exit command: %w", err)
	}
	// TODO: Add some ability to toggle syslog
	exitCommandArgs, err := specgenutil.CreateExitCommandArgs(storageConfig, runtimeConfig, logrus.IsLevelEnabled(logrus.DebugLevel), false, true)
	if err != nil {
		return nil, fmt.Errorf("constructing exit command for exec session: %w", err)
	}
	execConfig.ExitCommand = exitCommandArgs

	return execConfig, nil
}

func checkExecPreserveFDs(options entities.ExecOptions) error {
	if options.PreserveFDs > 0 {
		entries, err := os.ReadDir(processFileDescriptorsPath)
		if err != nil {
			return err
		}

		m := make(map[int]bool)
		for _, e := range entries {
			i, err := strconv.Atoi(e.Name())
			if err != nil {
				return fmt.Errorf("cannot parse %s in %s: %w", e.Name(), processFileDescriptorsPath, err)
			}
			m[i] = true
		}

		for i := 3; i < 3+int(options.PreserveFDs); i++ {
			if _, found := m[i]; !found {
				return errors.New("invalid --preserve-fds=N specified. Not enough FDs available")
			}
		}
	}
	return nil
}

func (ic *ContainerEngine) ContainerExec(ctx context.Context, nameOrID string, options entities.ExecOptions, streams define.AttachStreams) (int, error) {
	ec := define.ExecErrorCodeGeneric
	err := checkExecPreserveFDs(options)
	if err != nil {
		return ec, err
	}
	containers, err := getContainers(ic.Libpod, getContainersOptions{latest: options.Latest, names: []string{nameOrID}})
	if err != nil {
		return ec, err
	}
	if len(containers) != 1 {
		return ec, fmt.Errorf("%w: expected to find exactly one container but got %d", define.ErrInternal, len(containers))
	}
	ctr := containers[0]

	if options.Tty {
		util.ExecAddTERM(ctr.Env(), options.Envs)
	}

	execConfig, err := makeExecConfig(options, ic.Libpod)
	if err != nil {
		return ec, err
	}

	ec, err = terminal.ExecAttachCtr(ctx, ctr.Container, execConfig, &streams)
	return define.TranslateExecErrorToExitCode(ec, err), err
}

func (ic *ContainerEngine) ContainerExecDetached(ctx context.Context, nameOrID string, options entities.ExecOptions) (string, error) {
	err := checkExecPreserveFDs(options)
	if err != nil {
		return "", err
	}

	containers, err := getContainers(ic.Libpod, getContainersOptions{latest: options.Latest, names: []string{nameOrID}})
	if err != nil {
		return "", err
	}
	if len(containers) != 1 {
		return "", fmt.Errorf("%w: expected to find exactly one container but got %d", define.ErrInternal, len(containers))
	}
	ctr := containers[0]

	execConfig, err := makeExecConfig(options, ic.Libpod)
	if err != nil {
		return "", err
	}

	// Create and start the exec session
	id, err := ctr.ExecCreate(execConfig)
	if err != nil {
		return "", err
	}

	// TODO: we should try and retrieve exit code if this fails.
	if err := ctr.ExecStart(id); err != nil {
		_ = ctr.ExecRemove(id, true)
		return "", err
	}
	return id, nil
}

func (ic *ContainerEngine) ContainerStart(ctx context.Context, namesOrIds []string, options entities.ContainerStartOptions) ([]*entities.ContainerStartReport, error) {
	reports := []*entities.ContainerStartReport{}
	var exitCode = define.ExecErrorCodeGeneric
	containers, err := getContainers(ic.Libpod, getContainersOptions{all: options.All, latest: options.Latest, names: namesOrIds, filters: options.Filters})
	if err != nil {
		return nil, err
	}
	// There can only be one container if attach was used
	for i := range containers {
		ctr := containers[i]
		ctrState, err := ctr.State()
		if err != nil {
			return nil, err
		}
		ctrRunning := ctrState == define.ContainerStateRunning

		if options.Attach {
			err = terminal.StartAttachCtr(ctx, ctr.Container, options.Stdout, options.Stderr, options.Stdin, options.DetachKeys, options.SigProxy, !ctrRunning)
			if errors.Is(err, define.ErrDetach) {
				// User manually detached
				// Exit cleanly immediately
				reports = append(reports, &entities.ContainerStartReport{
					Id:       ctr.ID(),
					RawInput: ctr.rawInput,
					Err:      nil,
					ExitCode: 0,
				})
				return reports, nil
			}

			if errors.Is(err, define.ErrWillDeadlock) {
				logrus.Debugf("Deadlock error: %v", err)
				reports = append(reports, &entities.ContainerStartReport{
					Id:       ctr.ID(),
					RawInput: ctr.rawInput,
					Err:      err,
					ExitCode: define.ExitCode(err),
				})
				return reports, fmt.Errorf("attempting to start container %s would cause a deadlock; please run 'podman system renumber' to resolve", ctr.ID())
			}

			if ctrRunning {
				reports = append(reports, &entities.ContainerStartReport{
					Id:       ctr.ID(),
					RawInput: ctr.rawInput,
					Err:      nil,
					ExitCode: 0,
				})
				return reports, err
			}

			if err != nil {
				reports = append(reports, &entities.ContainerStartReport{
					Id:       ctr.ID(),
					RawInput: ctr.rawInput,
					Err:      err,
					ExitCode: exitCode,
				})
				if ctr.AutoRemove() {
					if _, _, err := ic.removeContainer(ctx, ctr.Container, entities.RmOptions{}); err != nil {
						logrus.Errorf("Removing container %s: %v", ctr.ID(), err)
					}
				}
				return reports, fmt.Errorf("unable to start container %s: %w", ctr.ID(), err)
			}

			exitCode = ic.GetContainerExitCode(ctx, ctr.Container)
			reports = append(reports, &entities.ContainerStartReport{
				Id:       ctr.ID(),
				RawInput: ctr.rawInput,
				Err:      err,
				ExitCode: exitCode,
			})
			return reports, nil
		} // end attach

		// Start the container if it's not running already.
		if !ctrRunning {
			// Handle non-attach start
			// If the container is in a pod, also set to recursively start dependencies
			report := &entities.ContainerStartReport{
				Id:       ctr.ID(),
				RawInput: ctr.rawInput,
				ExitCode: 125,
			}
			if err := ctr.Start(ctx, true); err != nil {
				report.Err = err
				if errors.Is(err, define.ErrWillDeadlock) {
					report.Err = fmt.Errorf("please run 'podman system renumber' to resolve deadlocks: %w", err)
					reports = append(reports, report)
					continue
				}
				report.Err = fmt.Errorf("unable to start container %q: %w", ctr.ID(), err)
				reports = append(reports, report)
				if ctr.AutoRemove() {
					if _, _, err := ic.removeContainer(ctx, ctr.Container, entities.RmOptions{}); err != nil {
						logrus.Errorf("Removing container %s: %v", ctr.ID(), err)
					}
				}
				continue
			}
			report.ExitCode = 0
			reports = append(reports, report)
		}
	}
	return reports, nil
}

func (ic *ContainerEngine) ContainerList(ctx context.Context, options entities.ContainerListOptions) ([]entities.ListContainer, error) {
	if options.Latest {
		options.Last = 1
	}
	return ps.GetContainerLists(ic.Libpod, options)
}

func (ic *ContainerEngine) ContainerListExternal(ctx context.Context) ([]entities.ListContainer, error) {
	return ps.GetExternalContainerLists(ic.Libpod)
}

// Diff provides changes to given container
func (ic *ContainerEngine) Diff(ctx context.Context, namesOrIDs []string, opts entities.DiffOptions) (*entities.DiffReport, error) {
	var (
		base   string
		parent string
	)
	if opts.Latest {
		ctnr, err := ic.Libpod.GetLatestContainer()
		if err != nil {
			return nil, fmt.Errorf("unable to get latest container: %w", err)
		}
		base = ctnr.ID()
	}
	if len(namesOrIDs) > 0 {
		base = namesOrIDs[0]
		if len(namesOrIDs) > 1 {
			parent = namesOrIDs[1]
		}
	}
	changes, err := ic.Libpod.GetDiff(parent, base, opts.Type)
	return &entities.DiffReport{Changes: changes}, err
}

func (ic *ContainerEngine) ContainerRun(ctx context.Context, opts entities.ContainerRunOptions) (*entities.ContainerRunReport, error) {
	removeContainer := func(ctr *libpod.Container, force bool) error {
		var timeout *uint
		if err := ic.Libpod.RemoveContainer(ctx, ctr, force, true, timeout); err != nil {
			logrus.Debugf("unable to remove container %s after failing to start and attach to it: %v", ctr.ID(), err)
			return err
		}
		return nil
	}

	warn, err := generate.CompleteSpec(ctx, ic.Libpod, opts.Spec)
	if err != nil {
		return nil, err
	}
	// Print warnings
	for _, w := range warn {
		fmt.Fprintf(os.Stderr, "%s\n", w)
	}

	if opts.Spec != nil && !reflect.ValueOf(opts.Spec).IsNil() {
		// If this is a checkpoint image, restore it.
		img, resolvedImageName := opts.Spec.GetImage()
		if img != nil && resolvedImageName != "" {
			imgData, err := img.Inspect(ctx, nil)
			if err != nil {
				return nil, err
			}
			if imgData != nil {
				_, isCheckpointImage := imgData.Annotations[define.CheckpointAnnotationRuntimeName]
				if isCheckpointImage {
					var restoreOptions entities.RestoreOptions
					restoreOptions.Name = opts.Spec.Name
					restoreOptions.Pod = opts.Spec.Pod
					responses, err := ic.ContainerRestore(ctx, []string{resolvedImageName}, restoreOptions)
					if err != nil {
						return nil, err
					}

					report := entities.ContainerRunReport{}
					for _, r := range responses {
						report.Id = r.Id
						report.ExitCode = 0
						if r.Err != nil {
							logrus.Errorf("Failed to restore checkpoint image %s: %v", resolvedImageName, r.Err)
							report.ExitCode = 126
						}
						if r.RawInput != "" {
							logrus.Errorf("Failed to restore checkpoint image %s: %v", resolvedImageName, r.RawInput)
							report.ExitCode = 126
						}
					}
					return &report, nil
				}
			}
		}
	}

	rtSpec, spec, optsN, err := generate.MakeContainer(ctx, ic.Libpod, opts.Spec, false, nil)
	if err != nil {
		return nil, err
	}
	ctr, err := generate.ExecuteCreate(ctx, ic.Libpod, rtSpec, spec, false, optsN...)
	if err != nil {
		return nil, err
	}

	if opts.CIDFile != "" {
		if err := util.CreateIDFile(opts.CIDFile, ctr.ID()); err != nil {
			// If you fail to create CIDFile then remove the container
			_ = removeContainer(ctr, true)
			return nil, err
		}
	}

	report := entities.ContainerRunReport{Id: ctr.ID()}

	if logrus.GetLevel() == logrus.DebugLevel {
		cgroupPath, err := ctr.CgroupPath()
		if err == nil {
			logrus.Debugf("container %q has CgroupParent %q", ctr.ID(), cgroupPath)
		}
	}
	if opts.Detach {
		// if the container was created as part of a pod, also start its dependencies, if any.
		if err := ctr.Start(ctx, true); err != nil {
			// This means the command did not exist
			report.ExitCode = define.ExitCode(err)
			if opts.Rm {
				if rmErr := removeContainer(ctr, true); rmErr != nil && !errors.Is(rmErr, define.ErrNoSuchCtr) {
					logrus.Errorf("Container %s failed to be removed", ctr.ID())
				}
			}
			return &report, err
		}

		return &report, nil
	}

	// if the container was created as part of a pod, also start its dependencies, if any.
	if err := terminal.StartAttachCtr(ctx, ctr, opts.OutputStream, opts.ErrorStream, opts.InputStream, opts.DetachKeys, opts.SigProxy, true); err != nil {
		// We've manually detached from the container
		// Do not perform cleanup, or wait for container exit code
		// Just exit immediately
		if errors.Is(err, define.ErrDetach) {
			report.ExitCode = 0
			return &report, nil
		}
		if opts.Rm {
			_ = removeContainer(ctr, true)
		}
		if errors.Is(err, define.ErrWillDeadlock) {
			logrus.Debugf("Deadlock error on %q: %v", ctr.ID(), err)
			report.ExitCode = define.ExitCode(err)
			return &report, fmt.Errorf("attempting to start container %s would cause a deadlock; please run 'podman system renumber' to resolve", ctr.ID())
		}
		report.ExitCode = define.ExitCode(err)
		return &report, err
	}
	report.ExitCode = ic.GetContainerExitCode(ctx, ctr)
	if opts.Rm && !ctr.ShouldRestart(ctx) {
		if err := removeContainer(ctr, false); err != nil {
			if errors.Is(err, define.ErrNoSuchCtr) ||
				errors.Is(err, define.ErrCtrRemoved) {
				logrus.Infof("Container %s was already removed, skipping --rm", ctr.ID())
			} else {
				logrus.Errorf("Removing container %s: %v", ctr.ID(), err)
			}
		}
	}
	return &report, nil
}

func (ic *ContainerEngine) GetContainerExitCode(ctx context.Context, ctr *libpod.Container) int {
	exitCode, err := ctr.Wait(ctx)
	if err != nil {
		logrus.Errorf("Waiting for container %s: %v", ctr.ID(), err)
		return define.ExecErrorCodeNotFound
	}
	return int(exitCode)
}

func (ic *ContainerEngine) ContainerLogs(ctx context.Context, namesOrIds []string, options entities.ContainerLogsOptions) error {
	if options.StdoutWriter == nil && options.StderrWriter == nil {
		return errors.New("no io.Writer set for container logs")
	}

	var wg sync.WaitGroup

	isPod := false
	for _, c := range namesOrIds {
		ctr, err := ic.Libpod.LookupContainer(c)
		if err != nil {
			return err
		}
		if ctr.IsInfra() {
			isPod = true
			break
		}
	}

	containers, err := getContainers(ic.Libpod, getContainersOptions{latest: options.Latest, isPod: isPod, names: namesOrIds})
	if err != nil {
		return err
	}

	logOpts := &logs.LogOptions{
		Multi:      len(containers) > 1,
		Details:    options.Details,
		Follow:     options.Follow,
		Since:      options.Since,
		Until:      options.Until,
		Tail:       options.Tail,
		Timestamps: options.Timestamps,
		Colors:     options.Colors,
		UseName:    options.Names,
		WaitGroup:  &wg,
	}

	chSize := len(containers)
	logChannel := make(chan *logs.LogLine, chSize)

	libpodContainers := make([]*libpod.Container, len(containers))
	for i := range containers {
		libpodContainers[i] = containers[i].Container
	}

	if err := ic.Libpod.Log(ctx, libpodContainers, logOpts, logChannel); err != nil {
		return err
	}

	go func() {
		wg.Wait()
		close(logChannel)
	}()

	for line := range logChannel {
		line.Write(options.StdoutWriter, options.StderrWriter, logOpts)
	}

	return nil
}

func (ic *ContainerEngine) ContainerCleanup(ctx context.Context, namesOrIds []string, options entities.ContainerCleanupOptions) ([]*entities.ContainerCleanupReport, error) {
	containers, err := getContainers(ic.Libpod, getContainersOptions{all: options.All, latest: options.Latest, names: namesOrIds})
	if err != nil {
		return nil, err
	}
	reports := []*entities.ContainerCleanupReport{}
	for _, ctr := range containers {
		var err error
		report := entities.ContainerCleanupReport{Id: ctr.ID(), RawInput: ctr.rawInput}

		if options.Exec != "" {
			if options.Remove {
				if err := ctr.ExecRemove(options.Exec, false); err != nil {
					return nil, err
				}
			} else {
				if err := ctr.ExecCleanup(options.Exec); err != nil {
					return nil, err
				}
			}
			return []*entities.ContainerCleanupReport{}, nil
		}

		if options.Remove && !ctr.ShouldRestart(ctx) {
			var timeout *uint
			err = ic.Libpod.RemoveContainer(ctx, ctr.Container, false, true, timeout)
			if err != nil {
				report.RmErr = fmt.Errorf("failed to clean up and remove container %v: %w", ctr.ID(), err)
			}
		} else {
			err := ctr.Cleanup(ctx)
			if err != nil {
				report.CleanErr = fmt.Errorf("failed to clean up container %v: %w", ctr.ID(), err)
			}
		}

		if options.RemoveImage {
			_, imageName := ctr.Image()
			imageEngine := ImageEngine{Libpod: ic.Libpod}
			_, rmErrors := imageEngine.Remove(ctx, []string{imageName}, entities.ImageRemoveOptions{})
			report.RmiErr = errorhandling.JoinErrors(rmErrors)
		}

		reports = append(reports, &report)
	}
	return reports, nil
}

func (ic *ContainerEngine) ContainerInit(ctx context.Context, namesOrIds []string, options entities.ContainerInitOptions) ([]*entities.ContainerInitReport, error) {
	containers, err := getContainers(ic.Libpod, getContainersOptions{all: options.All, latest: options.Latest, names: namesOrIds})
	if err != nil {
		return nil, err
	}
	reports := make([]*entities.ContainerInitReport, 0, len(containers))
	for _, ctr := range containers {
		report := entities.ContainerInitReport{Id: ctr.ID(), RawInput: ctr.rawInput}
		err := ctr.Init(ctx, ctr.PodID() != "")

		// If we're initializing all containers, ignore invalid state errors
		if options.All && errors.Is(err, define.ErrCtrStateInvalid) {
			err = nil
		}
		report.Err = err
		reports = append(reports, &report)
	}
	return reports, nil
}

func (ic *ContainerEngine) ContainerMount(ctx context.Context, nameOrIDs []string, options entities.ContainerMountOptions) ([]*entities.ContainerMountReport, error) {
	if os.Geteuid() != 0 {
		if driver := ic.Libpod.StorageConfig().GraphDriverName; driver != "vfs" {
			// Do not allow to mount a graphdriver that is not vfs if we are creating the userns as part
			// of the mount command.
			return nil, fmt.Errorf("cannot mount using driver %s in rootless mode", driver)
		}

		became, ret, err := rootless.BecomeRootInUserNS("")
		if err != nil {
			return nil, err
		}
		if became {
			os.Exit(ret)
		}
	}
	reports := []*entities.ContainerMountReport{}
	// Attempt to mount named containers directly from storage,
	// this will fail and code will fall through to removing the container from libpod.`
	names := []string{}
	for _, ctr := range nameOrIDs {
		report := entities.ContainerMountReport{Id: ctr}
		if report.Path, report.Err = ic.Libpod.MountStorageContainer(ctr); report.Err != nil {
			names = append(names, ctr)
		} else {
			reports = append(reports, &report)
		}
	}

	containers, err := getContainers(ic.Libpod, getContainersOptions{all: options.All, latest: options.Latest, names: names})
	if err != nil {
		return nil, err
	}
	for _, ctr := range containers {
		report := entities.ContainerMountReport{Id: ctr.ID()}
		report.Path, report.Err = ctr.Mount()
		reports = append(reports, &report)
	}
	if len(reports) > 0 {
		return reports, nil
	}

	storageCtrs, err := ic.Libpod.StorageContainers()
	if err != nil {
		return nil, err
	}

	for _, sctr := range storageCtrs {
		mounted, path, err := ic.Libpod.IsStorageContainerMounted(sctr.ID)
		if err != nil {
			return nil, err
		}

		var name string
		if len(sctr.Names) > 0 {
			name = sctr.Names[0]
		}
		if mounted {
			reports = append(reports, &entities.ContainerMountReport{
				Id:   sctr.ID,
				Name: name,
				Path: path,
			})
		}
	}

	// No containers were passed, so we send back what is mounted
	containers, err = getContainers(ic.Libpod, getContainersOptions{all: true})
	if err != nil {
		return nil, err
	}
	for _, ctr := range containers {
		mounted, path, err := ctr.Mounted()
		if err != nil {
			return nil, err
		}

		if mounted {
			reports = append(reports, &entities.ContainerMountReport{
				Id:   ctr.ID(),
				Name: ctr.Name(),
				Path: path,
			})
		}
	}

	return reports, nil
}

func (ic *ContainerEngine) ContainerUnmount(ctx context.Context, nameOrIDs []string, options entities.ContainerUnmountOptions) ([]*entities.ContainerUnmountReport, error) {
	reports := []*entities.ContainerUnmountReport{}
	names := []string{}
	if options.All {
		storageCtrs, err := ic.Libpod.StorageContainers()
		if err != nil {
			return nil, err
		}
		for _, sctr := range storageCtrs {
			mounted, _, _ := ic.Libpod.IsStorageContainerMounted(sctr.ID)
			if mounted {
				report := entities.ContainerUnmountReport{Id: sctr.ID}
				if _, report.Err = ic.Libpod.UnmountStorageContainer(sctr.ID, options.Force); report.Err != nil {
					if !errors.Is(report.Err, define.ErrCtrExists) {
						reports = append(reports, &report)
					}
				} else {
					reports = append(reports, &report)
				}
			}
		}
	}
	for _, ctr := range nameOrIDs {
		report := entities.ContainerUnmountReport{Id: ctr}
		if _, report.Err = ic.Libpod.UnmountStorageContainer(ctr, options.Force); report.Err != nil {
			names = append(names, ctr)
		} else {
			reports = append(reports, &report)
		}
	}
	containers, err := getContainers(ic.Libpod, getContainersOptions{all: options.All, latest: options.Latest, names: names})
	if err != nil {
		return nil, err
	}
	for _, ctr := range containers {
		state, err := ctr.State()
		if err != nil {
			logrus.Debugf("Error umounting container %s state: %s", ctr.ID(), err.Error())
			continue
		}
		if state == define.ContainerStateRunning {
			logrus.Debugf("Error umounting container %s, is running", ctr.ID())
			continue
		}

		report := entities.ContainerUnmountReport{Id: ctr.ID()}
		if err := ctr.Unmount(options.Force); err != nil {
			if options.All && errors.Is(err, storage.ErrLayerNotMounted) {
				logrus.Debugf("Error umounting container %s, storage.ErrLayerNotMounted", ctr.ID())
				continue
			}
			report.Err = fmt.Errorf("unmounting container %s: %w", ctr.ID(), err)
		}
		reports = append(reports, &report)
	}
	return reports, nil
}

// GetConfig returns a copy of the configuration used by the runtime
func (ic *ContainerEngine) Config(_ context.Context) (*config.Config, error) {
	return ic.Libpod.GetConfig()
}

func (ic *ContainerEngine) ContainerPort(ctx context.Context, nameOrID string, options entities.ContainerPortOptions) ([]*entities.ContainerPortReport, error) {
	containers, err := getContainers(ic.Libpod, getContainersOptions{all: options.All, latest: options.Latest, names: []string{nameOrID}})
	if err != nil {
		return nil, err
	}
	reports := []*entities.ContainerPortReport{}
	for _, con := range containers {
		state, err := con.State()
		if err != nil {
			return nil, err
		}
		if state != define.ContainerStateRunning {
			continue
		}
		portmappings, err := con.PortMappings()
		if err != nil {
			return nil, err
		}
		if len(portmappings) > 0 {
			reports = append(reports, &entities.ContainerPortReport{
				Id:    con.ID(),
				Ports: portmappings,
			})
		}
	}
	return reports, nil
}

// Shutdown Libpod engine
func (ic *ContainerEngine) Shutdown(_ context.Context) {
	shutdownSync.Do(func() {
		_ = ic.Libpod.Shutdown(false)
	})
}

func (ic *ContainerEngine) ContainerStats(ctx context.Context, namesOrIds []string, options entities.ContainerStatsOptions) (statsChan chan entities.ContainerStatsReport, err error) {
	if options.Interval < 1 {
		return nil, errors.New("invalid interval, must be a positive number greater zero")
	}
	if rootless.IsRootless() {
		unified, err := cgroups.IsCgroup2UnifiedMode()
		if err != nil {
			return nil, err
		}
		if !unified {
			return nil, errors.New("stats is not supported in rootless mode without cgroups v2")
		}
	}
	statsChan = make(chan entities.ContainerStatsReport, 1)

	var containerFunc func() ([]*libpod.Container, error)
	queryAll := false
	switch {
	case options.Latest:
		containerFunc = func() ([]*libpod.Container, error) {
			lastCtr, err := ic.Libpod.GetLatestContainer()
			if err != nil {
				return nil, err
			}
			return []*libpod.Container{lastCtr}, nil
		}
	case len(namesOrIds) > 0:
		containerFunc = func() ([]*libpod.Container, error) { return ic.Libpod.GetContainersByList(namesOrIds) }
	case options.All:
		queryAll = true
		containerFunc = ic.Libpod.GetAllContainers
	default:
		// queryAll is used to ignore errors when the container was removed between listing and
		// checking stats which we should do for running containers as well
		queryAll = true
		containerFunc = ic.Libpod.GetRunningContainers
	}

	go func() {
		defer close(statsChan)
		var (
			err            error
			containers     []*libpod.Container
			containerStats map[string]*define.ContainerStats
		)
		containerStats = make(map[string]*define.ContainerStats)

	stream: // label to flatten the scope
		select {
		case <-ctx.Done():
			// client cancelled
			logrus.Debugf("Container stats stopped: context cancelled")
			return
		default:
			// just fall through and do work
		}

		// Anonymous func to easily use the return values for streaming.
		computeStats := func() ([]define.ContainerStats, error) {
			containers, err = containerFunc()
			if err != nil {
				return nil, fmt.Errorf("unable to get list of containers: %w", err)
			}

			reportStats := []define.ContainerStats{}
			for _, ctr := range containers {
				stats, err := ctr.GetContainerStats(containerStats[ctr.ID()])
				if err != nil {
					if queryAll && (errors.Is(err, define.ErrCtrRemoved) || errors.Is(err, define.ErrNoSuchCtr) || errors.Is(err, define.ErrCtrStateInvalid)) {
						continue
					}
					return nil, err
				}

				containerStats[ctr.ID()] = stats
				reportStats = append(reportStats, *stats)
			}
			return reportStats, nil
		}

		report := entities.ContainerStatsReport{}
		report.Stats, report.Error = computeStats()
		statsChan <- report

		if report.Error != nil || !options.Stream {
			return
		}

		time.Sleep(time.Second * time.Duration(options.Interval))
		goto stream
	}()

	return statsChan, nil
}

// ShouldRestart returns whether the container should be restarted
func (ic *ContainerEngine) ShouldRestart(ctx context.Context, nameOrID string) (*entities.BoolReport, error) {
	ctr, err := ic.Libpod.LookupContainer(nameOrID)
	if err != nil {
		return nil, err
	}

	return &entities.BoolReport{Value: ctr.ShouldRestart(ctx)}, nil
}

// ContainerRename renames the given container.
func (ic *ContainerEngine) ContainerRename(ctx context.Context, nameOrID string, opts entities.ContainerRenameOptions) error {
	ctr, err := ic.Libpod.LookupContainer(nameOrID)
	if err != nil {
		return err
	}

	if _, err := ic.Libpod.RenameContainer(ctx, ctr, opts.NewName); err != nil {
		return err
	}

	return nil
}

func (ic *ContainerEngine) ContainerClone(ctx context.Context, ctrCloneOpts entities.ContainerCloneOptions) (*entities.ContainerCreateReport, error) {
	spec := specgen.NewSpecGenerator(ctrCloneOpts.Image, ctrCloneOpts.CreateOpts.RootFS)
	var c *libpod.Container
	c, _, err := generate.ConfigToSpec(ic.Libpod, spec, ctrCloneOpts.ID)
	if err != nil {
		return nil, err
	}

	if ctrCloneOpts.CreateOpts.Pod != "" {
		pod, err := ic.Libpod.LookupPod(ctrCloneOpts.CreateOpts.Pod)
		if err != nil {
			return nil, err
		}

		if len(spec.Networks) > 0 && pod.SharesNet() {
			logrus.Warning("resetting network config, cannot specify a network other than the pod's when sharing the net namespace")
			spec.Networks = nil
			spec.NetworkOptions = nil
		}

		allNamespaces := []struct {
			isShared bool
			value    *specgen.Namespace
		}{
			{pod.SharesPID(), &spec.PidNS},
			{pod.SharesNet(), &spec.NetNS},
			{pod.SharesCgroup(), &spec.CgroupNS},
			{pod.SharesIPC(), &spec.IpcNS},
			{pod.SharesUTS(), &spec.UtsNS},
		}

		printWarning := false
		for _, n := range allNamespaces {
			if n.isShared && !n.value.IsDefault() {
				*n.value = specgen.Namespace{NSMode: specgen.Default}
				printWarning = true
			}
		}
		if printWarning {
			logrus.Warning("At least one namespace was reset to the default configuration")
		}
	}

	err = specgenutil.FillOutSpecGen(spec, &ctrCloneOpts.CreateOpts, []string{})
	if err != nil {
		return nil, err
	}
	out, err := generate.CompleteSpec(ctx, ic.Libpod, spec)
	if err != nil {
		return nil, err
	}

	// if we do not pass term, running ctrs exit
	spec.Terminal = c.Terminal()

	// Print warnings
	if len(out) > 0 {
		for _, w := range out {
			fmt.Println("Could not properly complete the spec as expected:")
			fmt.Fprintf(os.Stderr, "%s\n", w)
		}
	}

	if len(ctrCloneOpts.CreateOpts.Name) > 0 {
		spec.Name = ctrCloneOpts.CreateOpts.Name
	} else {
		n := c.Name()
		_, err := ic.Libpod.LookupContainer(c.Name() + "-clone")
		if err == nil {
			n += "-clone"
		}
		spec.Name = generate.CheckName(ic.Libpod, n, true)
	}

	rtSpec, spec, opts, err := generate.MakeContainer(context.Background(), ic.Libpod, spec, true, c)
	if err != nil {
		return nil, err
	}
	ctr, err := generate.ExecuteCreate(ctx, ic.Libpod, rtSpec, spec, false, opts...)
	if err != nil {
		return nil, err
	}

	if ctrCloneOpts.Destroy {
		var time *uint
		err = ic.Libpod.RemoveContainer(context.Background(), c, ctrCloneOpts.Force, false, time)
		if err != nil {
			return nil, err
		}
	}

	if ctrCloneOpts.Run {
		if err := ctr.Start(ctx, true); err != nil {
			return nil, err
		}
	}

	return &entities.ContainerCreateReport{Id: ctr.ID()}, nil
}

// ContainerUpdate finds and updates the given container's cgroup config with the specified options
func (ic *ContainerEngine) ContainerUpdate(ctx context.Context, updateOptions *entities.ContainerUpdateOptions) (string, error) {
	err := specgen.WeightDevices(updateOptions.Specgen)
	if err != nil {
		return "", err
	}
	err = specgen.FinishThrottleDevices(updateOptions.Specgen)
	if err != nil {
		return "", err
	}
	containers, err := getContainers(ic.Libpod, getContainersOptions{names: []string{updateOptions.NameOrID}})
	if err != nil {
		return "", err
	}
	if len(containers) != 1 {
		return "", fmt.Errorf("container not found")
	}

	if err = containers[0].Update(updateOptions.Specgen.ResourceLimits); err != nil {
		return "", err
	}
	return containers[0].ID(), nil
}
