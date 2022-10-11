package abi

import (
	"context"
	"errors"
	"fmt"
	"os"
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

// getContainersAndInputByContext gets containers whether all, latest, or a slice of names/ids
// is specified.  It also returns a list of the corresponding input name used to lookup each container.
func getContainersAndInputByContext(all, latest, isPod bool, names []string, filters map[string][]string, runtime *libpod.Runtime) (ctrs []*libpod.Container, rawInput []string, err error) {
	var ctr *libpod.Container
	var filteredCtrs []*libpod.Container
	ctrs = []*libpod.Container{}
	filterFuncs := make([]libpod.ContainerFilter, 0, len(filters))

	switch {
	case len(filters) > 0:
		for k, v := range filters {
			generatedFunc, err := dfilters.GenerateContainerFilterFuncs(k, v, runtime)
			if err != nil {
				return nil, nil, err
			}
			filterFuncs = append(filterFuncs, generatedFunc)
		}
		ctrs, err = runtime.GetContainers(filterFuncs...)
		if err != nil {
			return nil, nil, err
		}
		rawInput = []string{}
		for _, candidate := range ctrs {
			if len(names) > 0 {
				for _, name := range names {
					if candidate.ID() == name || candidate.Name() == name {
						rawInput = append(rawInput, candidate.ID())
						filteredCtrs = append(filteredCtrs, candidate)
					}
				}
				ctrs = filteredCtrs
			} else {
				rawInput = append(rawInput, candidate.ID())
			}
		}
	case all:
		ctrs, err = runtime.GetAllContainers()
		if err == nil {
			for _, ctr := range ctrs {
				rawInput = append(rawInput, ctr.ID())
			}
		}
	case latest:
		if isPod {
			pod, err := runtime.GetLatestPod()
			if err == nil {
				podCtrs, err := pod.AllContainers()
				if err == nil {
					for _, c := range podCtrs {
						rawInput = append(rawInput, c.ID())
					}
					ctrs = podCtrs
				}
			}
		} else {
			ctr, err = runtime.GetLatestContainer()
			if err == nil {
				rawInput = append(rawInput, ctr.ID())
				ctrs = append(ctrs, ctr)
			}
		}
	default:
		for _, n := range names {
			ctr, e := runtime.LookupContainer(n)
			if e != nil {
				// Log all errors here, so callers don't need to.
				logrus.Debugf("Error looking up container %q: %v", n, e)
				if err == nil {
					err = e
				}
			} else {
				rawInput = append(rawInput, n)
				ctrs = append(ctrs, ctr)
			}
		}
	}
	return ctrs, rawInput, err
}

// getContainersByContext gets containers whether all, latest, or a slice of names/ids
// is specified.
func getContainersByContext(all, latest, isPod bool, names []string, runtime *libpod.Runtime) (ctrs []*libpod.Container, err error) {
	ctrs, _, err = getContainersAndInputByContext(all, latest, isPod, names, nil, runtime)
	return
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
	ctrs, err := getContainersByContext(false, options.Latest, false, namesOrIds, ic.Libpod)
	if err != nil {
		return nil, err
	}
	responses := make([]entities.WaitReport, 0, len(ctrs))
	for _, c := range ctrs {
		response := entities.WaitReport{Id: c.ID()}
		if options.Condition == nil {
			options.Condition = []define.ContainerStatus{define.ContainerStateStopped, define.ContainerStateExited}
		}
		exitCode, err := c.WaitForConditionWithInterval(ctx, options.Interval, options.Condition...)
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
	ctrs, rawInputs, err := getContainersAndInputByContext(options.All, options.Latest, false, namesOrIds, options.Filters, ic.Libpod)
	if err != nil {
		return nil, err
	}
	idToRawInput := map[string]string{}
	if len(rawInputs) == len(ctrs) {
		for i := range ctrs {
			idToRawInput[ctrs[i].ID()] = rawInputs[i]
		}
	}
	reports := make([]*entities.PauseUnpauseReport, 0, len(ctrs))
	for _, c := range ctrs {
		err := c.Pause()
		if err != nil && options.All && errors.Is(err, define.ErrCtrStateInvalid) {
			logrus.Debugf("Container %s is not running", c.ID())
			continue
		}
		reports = append(reports, &entities.PauseUnpauseReport{
			Id:       c.ID(),
			Err:      err,
			RawInput: idToRawInput[c.ID()],
		})
	}
	return reports, nil
}

func (ic *ContainerEngine) ContainerUnpause(ctx context.Context, namesOrIds []string, options entities.PauseUnPauseOptions) ([]*entities.PauseUnpauseReport, error) {
	ctrs, rawInputs, err := getContainersAndInputByContext(options.All, options.Latest, false, namesOrIds, options.Filters, ic.Libpod)
	if err != nil {
		return nil, err
	}
	idToRawInput := map[string]string{}
	if len(rawInputs) == len(ctrs) {
		for i := range ctrs {
			idToRawInput[ctrs[i].ID()] = rawInputs[i]
		}
	}
	reports := make([]*entities.PauseUnpauseReport, 0, len(ctrs))
	for _, c := range ctrs {
		err := c.Unpause()
		if err != nil && options.All && errors.Is(err, define.ErrCtrStateInvalid) {
			logrus.Debugf("Container %s is not paused", c.ID())
			continue
		}
		reports = append(reports, &entities.PauseUnpauseReport{
			Id:       c.ID(),
			Err:      err,
			RawInput: idToRawInput[c.ID()],
		})
	}
	return reports, nil
}
func (ic *ContainerEngine) ContainerStop(ctx context.Context, namesOrIds []string, options entities.StopOptions) ([]*entities.StopReport, error) {
	names := namesOrIds
	ctrs, rawInputs, err := getContainersAndInputByContext(options.All, options.Latest, false, names, options.Filters, ic.Libpod)
	if err != nil && !(options.Ignore && errors.Is(err, define.ErrNoSuchCtr)) {
		return nil, err
	}
	idToRawInput := map[string]string{}
	if len(rawInputs) == len(ctrs) {
		for i := range ctrs {
			idToRawInput[ctrs[i].ID()] = rawInputs[i]
		}
	}
	errMap, err := parallelctr.ContainerOp(ctx, ctrs, func(c *libpod.Container) error {
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
				logrus.Debugf("Container %s is either not created on runtime or is in a invalid state", c.ID())
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
		if options.All {
			report.RawInput = ctr.ID()
		} else {
			report.RawInput = idToRawInput[ctr.ID()]
		}
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
	ctrs, rawInputs, err := getContainersAndInputByContext(options.All, options.Latest, false, namesOrIds, nil, ic.Libpod)
	if err != nil {
		return nil, err
	}
	idToRawInput := map[string]string{}
	if len(rawInputs) == len(ctrs) {
		for i := range ctrs {
			idToRawInput[ctrs[i].ID()] = rawInputs[i]
		}
	}
	reports := make([]*entities.KillReport, 0, len(ctrs))
	for _, con := range ctrs {
		err := con.Kill(uint(sig))
		if options.All && errors.Is(err, define.ErrCtrStateInvalid) {
			logrus.Debugf("Container %s is not running", con.ID())
			continue
		}
		reports = append(reports, &entities.KillReport{
			Id:       con.ID(),
			Err:      err,
			RawInput: idToRawInput[con.ID()],
		})
	}
	return reports, nil
}

func (ic *ContainerEngine) ContainerRestart(ctx context.Context, namesOrIds []string, options entities.RestartOptions) ([]*entities.RestartReport, error) {
	var (
		ctrs      []*libpod.Container
		err       error
		rawInputs = []string{}
	)

	if options.Running {
		ctrs, err = ic.Libpod.GetRunningContainers()
		for _, candidate := range ctrs {
			rawInputs = append(rawInputs, candidate.ID())
		}

		if err != nil {
			return nil, err
		}
	} else {
		ctrs, rawInputs, err = getContainersAndInputByContext(options.All, options.Latest, false, namesOrIds, options.Filters, ic.Libpod)
		if err != nil {
			return nil, err
		}
	}
	idToRawInput := map[string]string{}
	if len(rawInputs) == len(ctrs) {
		for i := range ctrs {
			idToRawInput[ctrs[i].ID()] = rawInputs[i]
		}
	}
	reports := make([]*entities.RestartReport, 0, len(ctrs))
	for _, c := range ctrs {
		timeout := c.StopTimeout()
		if options.Timeout != nil {
			timeout = *options.Timeout
		}
		reports = append(reports, &entities.RestartReport{
			Id:       c.ID(),
			Err:      c.RestartWithTimeout(ctx, timeout),
			RawInput: idToRawInput[c.ID()],
		})
	}
	return reports, nil
}

func (ic *ContainerEngine) removeContainer(ctx context.Context, ctr *libpod.Container, options entities.RmOptions) error {
	err := ic.Libpod.RemoveContainer(ctx, ctr, options.Force, options.Volumes, options.Timeout)
	if err == nil {
		return nil
	}
	logrus.Debugf("Failed to remove container %s: %s", ctr.ID(), err.Error())
	if errors.Is(err, define.ErrNoSuchCtr) {
		// Ignore if the container does not exist (anymore) when either
		// it has been requested by the user of if the container is a
		// service one.  Service containers are removed along with its
		// pods which in turn are removed along with their infra
		// container.  Hence, there is an inherent race when removing
		// infra containers with service containers in parallel.
		if options.Ignore || ctr.IsService() {
			logrus.Debugf("Ignoring error (--allow-missing): %v", err)
			return nil
		}
	} else if errors.Is(err, define.ErrCtrRemoved) {
		return nil
	}
	return err
}

func (ic *ContainerEngine) ContainerRm(ctx context.Context, namesOrIds []string, options entities.RmOptions) ([]*reports.RmReport, error) {
	rmReports := []*reports.RmReport{}

	names := namesOrIds
	// Attempt to remove named containers directly from storage, if container is defined in libpod
	// this will fail and code will fall through to removing the container from libpod.`
	tmpNames := []string{}
	for _, ctr := range names {
		report := reports.RmReport{Id: ctr}
		report.Err = ic.Libpod.RemoveStorageContainer(ctr, options.Force)
		//nolint:gocritic
		if report.Err == nil {
			// remove container names that we successfully deleted
			rmReports = append(rmReports, &report)
		} else if errors.Is(report.Err, define.ErrNoSuchCtr) || errors.Is(report.Err, define.ErrCtrExists) {
			// There is still a potential this is a libpod container
			tmpNames = append(tmpNames, ctr)
		} else {
			if _, err := ic.Libpod.LookupContainer(ctr); errors.Is(err, define.ErrNoSuchCtr) {
				// remove container failed, but not a libpod container
				rmReports = append(rmReports, &report)
				continue
			}
			// attempt to remove as a libpod container
			tmpNames = append(tmpNames, ctr)
		}
	}
	names = tmpNames

	ctrs, rawInputs, err := getContainersAndInputByContext(options.All, options.Latest, false, names, options.Filters, ic.Libpod)
	if err != nil && !(options.Ignore && errors.Is(err, define.ErrNoSuchCtr)) {
		return nil, err
	}
	idToRawInput := map[string]string{}
	if len(rawInputs) == len(ctrs) {
		for i := range ctrs {
			idToRawInput[ctrs[i].ID()] = rawInputs[i]
		}
	}
	if err != nil && !(options.Ignore && errors.Is(err, define.ErrNoSuchCtr)) {
		// Failed to get containers. If force is specified, get the containers ID
		// and evict them
		if !options.Force {
			return nil, err
		}

		for _, ctr := range names {
			logrus.Debugf("Evicting container %q", ctr)
			report := reports.RmReport{
				Id:       ctr,
				RawInput: idToRawInput[ctr],
			}
			_, err := ic.Libpod.EvictContainer(ctx, ctr, options.Volumes)
			if err != nil {
				if options.Ignore && errors.Is(err, define.ErrNoSuchCtr) {
					logrus.Debugf("Ignoring error (--allow-missing): %v", err)
					rmReports = append(rmReports, &report)
					continue
				}
				report.Err = err
				rmReports = append(rmReports, &report)
				continue
			}
			rmReports = append(rmReports, &report)
		}
		return rmReports, nil
	}

	alreadyRemoved := make(map[string]bool)
	addReports := func(newReports []*reports.RmReport) {
		for i := range newReports {
			alreadyRemoved[newReports[i].Id] = true
			rmReports = append(rmReports, newReports[i])
		}
	}
	if !options.All && options.Depend {
		// Add additional containers based on dependencies to container map
		for _, ctr := range ctrs {
			if alreadyRemoved[ctr.ID()] {
				continue
			}
			reports, err := ic.Libpod.RemoveDepend(ctx, ctr, options.Force, options.Volumes, options.Timeout)
			if err != nil {
				return rmReports, err
			}
			addReports(reports)
		}
		return rmReports, nil
	}

	// If --all is set, make sure to remove the infra containers'
	// dependencies before doing the parallel removal below.
	if options.All {
		for _, ctr := range ctrs {
			if alreadyRemoved[ctr.ID()] || !ctr.IsInfra() {
				continue
			}
			reports, err := ic.Libpod.RemoveDepend(ctx, ctr, options.Force, options.Volumes, options.Timeout)
			if err != nil {
				return rmReports, err
			}
			addReports(reports)
		}
	}

	errMap, err := parallelctr.ContainerOp(ctx, ctrs, func(c *libpod.Container) error {
		if alreadyRemoved[c.ID()] {
			return nil
		}
		return ic.removeContainer(ctx, c, options)
	})
	if err != nil {
		return nil, err
	}
	for ctr, err := range errMap {
		if alreadyRemoved[ctr.ID()] {
			continue
		}
		report := new(reports.RmReport)
		report.Id = ctr.ID()
		report.Err = err
		report.RawInput = idToRawInput[ctr.ID()]
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
	coptions := buildah.CommitOptions{
		SignaturePolicyPath:   rtc.Engine.SignaturePolicyPath,
		ReportWriter:          options.Writer,
		SystemContext:         sc,
		PreferredManifestType: mimeType,
	}
	opts := libpod.ContainerCommitOptions{
		CommitOptions:  coptions,
		Pause:          options.Pause,
		IncludeVolumes: options.IncludeVolumes,
		Message:        options.Message,
		Changes:        options.Changes,
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
	var (
		ctrs      []*libpod.Container
		rawInputs []string
		err       error
	)
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

	idToRawInput := map[string]string{}
	if options.All {
		running := func(c *libpod.Container) bool {
			state, _ := c.State()
			return state == define.ContainerStateRunning
		}
		ctrs, err = ic.Libpod.GetContainers(running)
		if err != nil {
			return nil, err
		}
	} else {
		ctrs, rawInputs, err = getContainersAndInputByContext(false, options.Latest, false, namesOrIds, nil, ic.Libpod)
		if err != nil {
			return nil, err
		}
		if len(rawInputs) == len(ctrs) {
			for i := range ctrs {
				idToRawInput[ctrs[i].ID()] = rawInputs[i]
			}
		}
	}
	reports := make([]*entities.CheckpointReport, 0, len(ctrs))
	for _, c := range ctrs {
		criuStatistics, runtimeCheckpointDuration, err := c.Checkpoint(ctx, checkOpts)
		reports = append(reports, &entities.CheckpointReport{
			Err:             err,
			Id:              c.ID(),
			RawInput:        idToRawInput[c.ID()],
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
		ctrs, err = ic.Libpod.GetContainers(filterFuncs...)
	case options.Latest:
		ctrs, err = getContainersByContext(false, options.Latest, false, namesOrIds, ic.Libpod)
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
	ctrs, err := getContainersByContext(false, options.Latest, false, []string{nameOrID}, ic.Libpod)
	if err != nil {
		return err
	}
	ctr := ctrs[0]
	conState, err := ctr.State()
	if err != nil {
		return fmt.Errorf("unable to determine state of %s: %w", ctr.ID(), err)
	}
	if conState != define.ContainerStateRunning {
		return fmt.Errorf("you can only attach to running containers")
	}

	// If the container is in a pod, also set to recursively start dependencies
	err = terminal.StartAttachCtr(ctx, ctr, options.Stdout, options.Stderr, options.Stdin, options.DetachKeys, options.SigProxy, false)
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
		entries, err := os.ReadDir("/proc/self/fd")
		if err != nil {
			return err
		}

		m := make(map[int]bool)
		for _, e := range entries {
			i, err := strconv.Atoi(e.Name())
			if err != nil {
				return fmt.Errorf("cannot parse %s in /proc/self/fd: %w", e.Name(), err)
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
	ctrs, err := getContainersByContext(false, options.Latest, false, []string{nameOrID}, ic.Libpod)
	if err != nil {
		return ec, err
	}
	ctr := ctrs[0]

	execConfig, err := makeExecConfig(options, ic.Libpod)
	if err != nil {
		return ec, err
	}

	ec, err = terminal.ExecAttachCtr(ctx, ctr, execConfig, &streams)
	return define.TranslateExecErrorToExitCode(ec, err), err
}

func (ic *ContainerEngine) ContainerExecDetached(ctx context.Context, nameOrID string, options entities.ExecOptions) (string, error) {
	err := checkExecPreserveFDs(options)
	if err != nil {
		return "", err
	}
	ctrs, err := getContainersByContext(false, options.Latest, false, []string{nameOrID}, ic.Libpod)
	if err != nil {
		return "", err
	}
	ctr := ctrs[0]

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
		return "", err
	}
	return id, nil
}

func (ic *ContainerEngine) ContainerStart(ctx context.Context, namesOrIds []string, options entities.ContainerStartOptions) ([]*entities.ContainerStartReport, error) {
	reports := []*entities.ContainerStartReport{}
	var exitCode = define.ExecErrorCodeGeneric
	ctrs, rawInputs, err := getContainersAndInputByContext(options.All, options.Latest, false, namesOrIds, options.Filters, ic.Libpod)
	if err != nil {
		return nil, err
	}
	idToRawInput := map[string]string{}
	if len(rawInputs) == len(ctrs) {
		for i := range ctrs {
			idToRawInput[ctrs[i].ID()] = rawInputs[i]
		}
	}
	// There can only be one container if attach was used
	for i := range ctrs {
		ctr := ctrs[i]
		ctrState, err := ctr.State()
		if err != nil {
			return nil, err
		}
		ctrRunning := ctrState == define.ContainerStateRunning

		if options.Attach {
			err = terminal.StartAttachCtr(ctx, ctr, options.Stdout, options.Stderr, options.Stdin, options.DetachKeys, options.SigProxy, !ctrRunning)
			if errors.Is(err, define.ErrDetach) {
				// User manually detached
				// Exit cleanly immediately
				reports = append(reports, &entities.ContainerStartReport{
					Id:       ctr.ID(),
					RawInput: idToRawInput[ctr.ID()],
					Err:      nil,
					ExitCode: 0,
				})
				return reports, nil
			}

			if errors.Is(err, define.ErrWillDeadlock) {
				logrus.Debugf("Deadlock error: %v", err)
				reports = append(reports, &entities.ContainerStartReport{
					Id:       ctr.ID(),
					RawInput: idToRawInput[ctr.ID()],
					Err:      err,
					ExitCode: define.ExitCode(err),
				})
				return reports, fmt.Errorf("attempting to start container %s would cause a deadlock; please run 'podman system renumber' to resolve", ctr.ID())
			}

			if ctrRunning {
				reports = append(reports, &entities.ContainerStartReport{
					Id:       ctr.ID(),
					RawInput: idToRawInput[ctr.ID()],
					Err:      nil,
					ExitCode: 0,
				})
				return reports, err
			}

			if err != nil {
				reports = append(reports, &entities.ContainerStartReport{
					Id:       ctr.ID(),
					RawInput: idToRawInput[ctr.ID()],
					Err:      err,
					ExitCode: exitCode,
				})
				if ctr.AutoRemove() {
					if err := ic.removeContainer(ctx, ctr, entities.RmOptions{}); err != nil {
						logrus.Errorf("Removing container %s: %v", ctr.ID(), err)
					}
				}
				return reports, fmt.Errorf("unable to start container %s: %w", ctr.ID(), err)
			}

			exitCode = ic.GetContainerExitCode(ctx, ctr)
			reports = append(reports, &entities.ContainerStartReport{
				Id:       ctr.ID(),
				RawInput: idToRawInput[ctr.ID()],
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
				RawInput: idToRawInput[ctr.ID()],
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
					if err := ic.removeContainer(ctx, ctr, entities.RmOptions{}); err != nil {
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

	rtSpec, spec, optsN, err := generate.MakeContainer(ctx, ic.Libpod, opts.Spec, false, nil)
	if err != nil {
		return nil, err
	}
	ctr, err := generate.ExecuteCreate(ctx, ic.Libpod, rtSpec, spec, false, optsN...)
	if err != nil {
		return nil, err
	}

	if opts.CIDFile != "" {
		if err := util.CreateCidFile(opts.CIDFile, ctr.ID()); err != nil {
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

func (ic *ContainerEngine) ContainerLogs(ctx context.Context, containers []string, options entities.ContainerLogsOptions) error {
	if options.StdoutWriter == nil && options.StderrWriter == nil {
		return errors.New("no io.Writer set for container logs")
	}

	var wg sync.WaitGroup

	isPod := false
	for _, c := range containers {
		ctr, err := ic.Libpod.LookupContainer(c)
		if err != nil {
			return err
		}
		if ctr.IsInfra() {
			isPod = true
			break
		}
	}

	ctrs, err := getContainersByContext(false, options.Latest, isPod, containers, ic.Libpod)
	if err != nil {
		return err
	}

	logOpts := &logs.LogOptions{
		Multi:      len(ctrs) > 1,
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

	chSize := len(ctrs) * int(options.Tail)
	if chSize <= 0 {
		chSize = 1
	}
	logChannel := make(chan *logs.LogLine, chSize)

	if err := ic.Libpod.Log(ctx, ctrs, logOpts, logChannel); err != nil {
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
	ctrs, rawInputs, err := getContainersAndInputByContext(options.All, options.Latest, false, namesOrIds, nil, ic.Libpod)
	if err != nil {
		return nil, err
	}
	idToRawInput := map[string]string{}
	if len(rawInputs) == len(ctrs) {
		for i := range ctrs {
			idToRawInput[ctrs[i].ID()] = rawInputs[i]
		}
	}
	reports := []*entities.ContainerCleanupReport{}
	for _, ctr := range ctrs {
		var err error
		report := entities.ContainerCleanupReport{Id: ctr.ID(), RawInput: idToRawInput[ctr.ID()]}

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
			err = ic.Libpod.RemoveContainer(ctx, ctr, false, true, timeout)
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
	ctrs, rawInputs, err := getContainersAndInputByContext(options.All, options.Latest, false, namesOrIds, nil, ic.Libpod)
	if err != nil {
		return nil, err
	}
	idToRawInput := map[string]string{}
	if len(rawInputs) == len(ctrs) {
		for i := range ctrs {
			idToRawInput[ctrs[i].ID()] = rawInputs[i]
		}
	}
	reports := make([]*entities.ContainerInitReport, 0, len(ctrs))
	for _, ctr := range ctrs {
		report := entities.ContainerInitReport{Id: ctr.ID(), RawInput: idToRawInput[ctr.ID()]}
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

	ctrs, err := getContainersByContext(options.All, options.Latest, false, names, ic.Libpod)
	if err != nil {
		return nil, err
	}
	for _, ctr := range ctrs {
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
	ctrs, err = getContainersByContext(true, false, false, []string{}, ic.Libpod)
	if err != nil {
		return nil, err
	}
	for _, ctr := range ctrs {
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
	ctrs, err := getContainersByContext(options.All, options.Latest, false, names, ic.Libpod)
	if err != nil {
		return nil, err
	}
	for _, ctr := range ctrs {
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
	ctrs, err := getContainersByContext(options.All, options.Latest, false, []string{nameOrID}, ic.Libpod)
	if err != nil {
		return nil, err
	}
	reports := []*entities.ContainerPortReport{}
	for _, con := range ctrs {
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

	containerFunc := ic.Libpod.GetRunningContainers
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
	default:
		// No containers, no latest -> query all!
		queryAll = true
		containerFunc = ic.Libpod.GetAllContainers
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
					if errors.Is(err, cgroups.ErrCgroupV1Rootless) {
						err = cgroups.ErrCgroupV1Rootless
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
	ctrs, err := getContainersByContext(false, false, false, []string{updateOptions.NameOrID}, ic.Libpod)
	if err != nil {
		return "", err
	}
	if len(ctrs) != 1 {
		return "", fmt.Errorf("container not found")
	}

	if err = ctrs[0].Update(updateOptions.Specgen.ResourceLimits); err != nil {
		return "", err
	}
	return ctrs[0].ID(), nil
}
