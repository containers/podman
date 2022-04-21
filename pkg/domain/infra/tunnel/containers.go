package tunnel

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/containers/common/pkg/config"
	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/libpod/events"
	"github.com/containers/podman/v4/pkg/api/handlers"
	"github.com/containers/podman/v4/pkg/bindings/containers"
	"github.com/containers/podman/v4/pkg/bindings/images"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/domain/entities/reports"
	"github.com/containers/podman/v4/pkg/errorhandling"
	"github.com/containers/podman/v4/pkg/specgen"
	"github.com/containers/podman/v4/pkg/util"
	"github.com/containers/storage/types"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func (ic *ContainerEngine) ContainerRunlabel(ctx context.Context, label string, image string, args []string, options entities.ContainerRunlabelOptions) error {
	return errors.New("not implemented")
}

func (ic *ContainerEngine) ContainerExists(ctx context.Context, nameOrID string, options entities.ContainerExistsOptions) (*entities.BoolReport, error) {
	exists, err := containers.Exists(ic.ClientCtx, nameOrID, new(containers.ExistsOptions).WithExternal(options.External))
	return &entities.BoolReport{Value: exists}, err
}

func (ic *ContainerEngine) ContainerWait(ctx context.Context, namesOrIds []string, opts entities.WaitOptions) ([]entities.WaitReport, error) {
	cons, err := getContainersByContext(ic.ClientCtx, false, false, namesOrIds)
	if err != nil {
		return nil, err
	}
	responses := make([]entities.WaitReport, 0, len(cons))
	options := new(containers.WaitOptions).WithCondition(opts.Condition).WithInterval(opts.Interval.String())
	for _, c := range cons {
		response := entities.WaitReport{Id: c.ID}
		exitCode, err := containers.Wait(ic.ClientCtx, c.ID, options)
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
	ctrs, err := getContainersByContext(ic.ClientCtx, options.All, false, namesOrIds)
	if err != nil {
		return nil, err
	}
	reports := make([]*entities.PauseUnpauseReport, 0, len(ctrs))
	for _, c := range ctrs {
		err := containers.Pause(ic.ClientCtx, c.ID, nil)
		if err != nil && options.All && errors.Cause(err).Error() == define.ErrCtrStateInvalid.Error() {
			logrus.Debugf("Container %s is not running", c.ID)
			continue
		}
		reports = append(reports, &entities.PauseUnpauseReport{Id: c.ID, Err: err})
	}
	return reports, nil
}

func (ic *ContainerEngine) ContainerUnpause(ctx context.Context, namesOrIds []string, options entities.PauseUnPauseOptions) ([]*entities.PauseUnpauseReport, error) {
	reports := []*entities.PauseUnpauseReport{}
	ctrs, err := getContainersByContext(ic.ClientCtx, options.All, false, namesOrIds)
	if err != nil {
		return nil, err
	}
	for _, c := range ctrs {
		err := containers.Unpause(ic.ClientCtx, c.ID, nil)
		if err != nil && options.All && errors.Cause(err).Error() == define.ErrCtrStateInvalid.Error() {
			logrus.Debugf("Container %s is not paused", c.ID)
			continue
		}
		reports = append(reports, &entities.PauseUnpauseReport{Id: c.ID, Err: err})
	}
	return reports, nil
}

func (ic *ContainerEngine) ContainerStop(ctx context.Context, namesOrIds []string, opts entities.StopOptions) ([]*entities.StopReport, error) {
	reports := []*entities.StopReport{}
	ctrs, rawInputs, err := getContainersAndInputByContext(ic.ClientCtx, opts.All, opts.Ignore, namesOrIds)
	if err != nil {
		return nil, err
	}
	ctrMap := map[string]string{}
	for i := range ctrs {
		ctrMap[ctrs[i].ID] = rawInputs[i]
	}
	options := new(containers.StopOptions).WithIgnore(opts.Ignore)
	if to := opts.Timeout; to != nil {
		options.WithTimeout(*to)
	}
	for _, c := range ctrs {
		report := entities.StopReport{
			Id:       c.ID,
			RawInput: ctrMap[c.ID],
		}
		if err = containers.Stop(ic.ClientCtx, c.ID, options); err != nil {
			// These first two are considered non-fatal under the right conditions
			if errors.Cause(err).Error() == define.ErrCtrStopped.Error() {
				logrus.Debugf("Container %s is already stopped", c.ID)
				reports = append(reports, &report)
				continue
			} else if opts.All && errors.Cause(err).Error() == define.ErrCtrStateInvalid.Error() {
				logrus.Debugf("Container %s is not running, could not stop", c.ID)
				reports = append(reports, &report)
				continue
			}

			// TODO we need to associate errors returned by http with common
			// define.errors so that we can equity tests. this will allow output
			// to be the same as the native client
			report.Err = err
			reports = append(reports, &report)
			continue
		}
		reports = append(reports, &report)
	}
	return reports, nil
}

func (ic *ContainerEngine) ContainerKill(ctx context.Context, namesOrIds []string, opts entities.KillOptions) ([]*entities.KillReport, error) {
	ctrs, rawInputs, err := getContainersAndInputByContext(ic.ClientCtx, opts.All, false, namesOrIds)
	if err != nil {
		return nil, err
	}
	ctrMap := map[string]string{}
	for i := range ctrs {
		ctrMap[ctrs[i].ID] = rawInputs[i]
	}
	options := new(containers.KillOptions).WithSignal(opts.Signal)
	reports := make([]*entities.KillReport, 0, len(ctrs))
	for _, c := range ctrs {
		err := containers.Kill(ic.ClientCtx, c.ID, options)
		if err != nil && opts.All && errors.Cause(err).Error() == define.ErrCtrStateInvalid.Error() {
			logrus.Debugf("Container %s is not running", c.ID)
			continue
		}
		reports = append(reports, &entities.KillReport{
			Id:       c.ID,
			Err:      err,
			RawInput: ctrMap[c.ID],
		})
	}
	return reports, nil
}

func (ic *ContainerEngine) ContainerRestart(ctx context.Context, namesOrIds []string, opts entities.RestartOptions) ([]*entities.RestartReport, error) {
	var (
		reports = []*entities.RestartReport{}
	)
	options := new(containers.RestartOptions)
	if to := opts.Timeout; to != nil {
		options.WithTimeout(int(*to))
	}
	ctrs, err := getContainersByContext(ic.ClientCtx, opts.All, false, namesOrIds)
	if err != nil {
		return nil, err
	}
	for _, c := range ctrs {
		if opts.Running && c.State != define.ContainerStateRunning.String() {
			continue
		}
		reports = append(reports, &entities.RestartReport{
			Id:  c.ID,
			Err: containers.Restart(ic.ClientCtx, c.ID, options),
		})
	}
	return reports, nil
}

func (ic *ContainerEngine) ContainerRm(ctx context.Context, namesOrIds []string, opts entities.RmOptions) ([]*reports.RmReport, error) {
	// TODO there is no endpoint for container eviction.  Need to discuss
	options := new(containers.RemoveOptions).WithForce(opts.Force).WithVolumes(opts.Volumes).WithIgnore(opts.Ignore).WithDepend(opts.Depend)
	if opts.Timeout != nil {
		options = options.WithTimeout(*opts.Timeout)
	}

	toRemove := []string{}
	alreadyRemoved := make(map[string]bool) // Avoids trying to remove already removed containers
	if opts.All {
		ctrs, err := getContainersByContext(ic.ClientCtx, opts.All, opts.Ignore, nil)
		if err != nil {
			return nil, err
		}
		for _, c := range ctrs {
			toRemove = append(toRemove, c.ID)
		}
	} else {
		for _, ctr := range namesOrIds {
			// NOTE that we set ignore=true here to support
			// removing external containers (e.g., Buildah
			// containers). If we don't find the container,
			// we'll use the raw input provided by the user
			// instead of the ID. Since this can only happen
			// with external containers, it poses no threat
			// to the `alreadyRemoved` checks below.
			ctrs, err := getContainersByContext(ic.ClientCtx, false, true, []string{ctr})
			if err != nil {
				return nil, err
			}
			id := ctr
			if len(ctrs) == 1 {
				id = ctrs[0].ID
			}
			toRemove = append(toRemove, id)
		}
	}

	rmReports := make([]*reports.RmReport, 0, len(toRemove))
	for _, nameOrID := range toRemove {
		if alreadyRemoved[nameOrID] {
			continue
		}
		newReports, err := containers.Remove(ic.ClientCtx, nameOrID, options)
		if err != nil {
			rmReports = append(rmReports, &reports.RmReport{Id: nameOrID, Err: err})
			continue
		}
		for i := range newReports {
			alreadyRemoved[newReports[i].Id] = true
			rmReports = append(rmReports, newReports[i])
		}
	}
	return rmReports, nil
}

func (ic *ContainerEngine) ContainerPrune(ctx context.Context, opts entities.ContainerPruneOptions) ([]*reports.PruneReport, error) {
	options := new(containers.PruneOptions).WithFilters(opts.Filters)
	return containers.Prune(ic.ClientCtx, options)
}

func (ic *ContainerEngine) ContainerInspect(ctx context.Context, namesOrIds []string, opts entities.InspectOptions) ([]*entities.ContainerInspectReport, []error, error) {
	var (
		reports = make([]*entities.ContainerInspectReport, 0, len(namesOrIds))
		errs    = []error{}
	)
	options := new(containers.InspectOptions).WithSize(opts.Size)
	for _, name := range namesOrIds {
		inspect, err := containers.Inspect(ic.ClientCtx, name, options)
		if err != nil {
			errModel, ok := err.(*errorhandling.ErrorModel)
			if !ok {
				return nil, nil, err
			}
			if errModel.ResponseCode == 404 {
				errs = append(errs, errors.Errorf("no such container %q", name))
				continue
			}
			return nil, nil, err
		}
		reports = append(reports, &entities.ContainerInspectReport{InspectContainerData: inspect})
	}
	return reports, errs, nil
}

func (ic *ContainerEngine) ContainerTop(ctx context.Context, opts entities.TopOptions) (*entities.StringSliceReport, error) {
	switch {
	case opts.Latest:
		return nil, errors.New("latest is not supported")
	case opts.NameOrID == "":
		return nil, errors.New("NameOrID must be specified")
	}
	options := new(containers.TopOptions).WithDescriptors(opts.Descriptors)
	topOutput, err := containers.Top(ic.ClientCtx, opts.NameOrID, options)
	if err != nil {
		return nil, err
	}
	return &entities.StringSliceReport{Value: topOutput}, nil
}

func (ic *ContainerEngine) ContainerCommit(ctx context.Context, nameOrID string, opts entities.CommitOptions) (*entities.CommitReport, error) {
	var (
		repo string
		tag  = "latest"
	)
	if len(opts.ImageName) > 0 {
		ref, err := reference.Parse(opts.ImageName)
		if err != nil {
			return nil, errors.Wrapf(err, "error parsing reference %q", opts.ImageName)
		}
		if t, ok := ref.(reference.Tagged); ok {
			tag = t.Tag()
		}
		if r, ok := ref.(reference.Named); ok {
			repo = r.Name()
		}
		if len(repo) < 1 {
			return nil, errors.Errorf("invalid image name %q", opts.ImageName)
		}
	}
	options := new(containers.CommitOptions).WithAuthor(opts.Author).WithChanges(opts.Changes).WithComment(opts.Message).WithSquash(opts.Squash)
	options.WithFormat(opts.Format).WithPause(opts.Pause).WithRepo(repo).WithTag(tag)
	response, err := containers.Commit(ic.ClientCtx, nameOrID, options)
	if err != nil {
		return nil, err
	}
	return &entities.CommitReport{Id: response.ID}, nil
}

func (ic *ContainerEngine) ContainerExport(ctx context.Context, nameOrID string, options entities.ContainerExportOptions) error {
	var (
		err error
		w   io.Writer
	)
	if len(options.Output) > 0 {
		w, err = os.Create(options.Output)
		if err != nil {
			return err
		}
	}
	return containers.Export(ic.ClientCtx, nameOrID, w, nil)
}

func (ic *ContainerEngine) ContainerCheckpoint(ctx context.Context, namesOrIds []string, opts entities.CheckpointOptions) ([]*entities.CheckpointReport, error) {
	options := new(containers.CheckpointOptions)
	options.WithFileLocks(opts.FileLocks)
	options.WithIgnoreRootfs(opts.IgnoreRootFS)
	options.WithKeep(opts.Keep)
	options.WithExport(opts.Export)
	options.WithCreateImage(opts.CreateImage)
	options.WithTCPEstablished(opts.TCPEstablished)
	options.WithPrintStats(opts.PrintStats)
	options.WithPreCheckpoint(opts.PreCheckPoint)
	options.WithLeaveRunning(opts.LeaveRunning)
	options.WithWithPrevious(opts.WithPrevious)

	var (
		err  error
		ctrs = []entities.ListContainer{}
	)

	if opts.All {
		allCtrs, err := getContainersByContext(ic.ClientCtx, true, false, []string{})
		if err != nil {
			return nil, err
		}
		// narrow the list to running only
		for _, c := range allCtrs {
			if c.State == define.ContainerStateRunning.String() {
				ctrs = append(ctrs, c)
			}
		}
	} else {
		ctrs, err = getContainersByContext(ic.ClientCtx, false, false, namesOrIds)
		if err != nil {
			return nil, err
		}
	}
	reports := make([]*entities.CheckpointReport, 0, len(ctrs))
	for _, c := range ctrs {
		report, err := containers.Checkpoint(ic.ClientCtx, c.ID, options)
		if err != nil {
			reports = append(reports, &entities.CheckpointReport{Id: c.ID, Err: err})
		} else {
			reports = append(reports, report)
		}
	}
	return reports, nil
}

func (ic *ContainerEngine) ContainerRestore(ctx context.Context, namesOrIds []string, opts entities.RestoreOptions) ([]*entities.RestoreReport, error) {
	if opts.ImportPrevious != "" {
		return nil, fmt.Errorf("--import-previous is not supported on the remote client")
	}

	options := new(containers.RestoreOptions)
	options.WithFileLocks(opts.FileLocks)
	options.WithIgnoreRootfs(opts.IgnoreRootFS)
	options.WithIgnoreVolumes(opts.IgnoreVolumes)
	options.WithIgnoreStaticIP(opts.IgnoreStaticIP)
	options.WithIgnoreStaticMAC(opts.IgnoreStaticMAC)
	options.WithKeep(opts.Keep)
	options.WithName(opts.Name)
	options.WithTCPEstablished(opts.TCPEstablished)
	options.WithPod(opts.Pod)
	options.WithPrintStats(opts.PrintStats)
	options.WithPublishPorts(opts.PublishPorts)

	if opts.Import != "" {
		options.WithImportArchive(opts.Import)
		report, err := containers.Restore(ic.ClientCtx, "", options)
		return []*entities.RestoreReport{report}, err
	}

	var (
		ids = []string{}
	)
	if opts.All {
		allCtrs, err := getContainersByContext(ic.ClientCtx, true, false, []string{})
		if err != nil {
			return nil, err
		}
		// narrow the list to exited only
		for _, c := range allCtrs {
			if c.State == define.ContainerStateExited.String() {
				ids = append(ids, c.ID)
			}
		}
	} else {
		getImageOptions := new(images.GetOptions).WithSize(false)
		hostInfo, err := ic.Info(context.Background())
		if err != nil {
			return nil, err
		}

		for _, nameOrID := range namesOrIds {
			ctrData, _, err := ic.ContainerInspect(ic.ClientCtx, []string{nameOrID}, entities.InspectOptions{})
			if err == nil && len(ctrData) > 0 {
				ids = append(ids, ctrData[0].ID)
			} else {
				// If container was not found, check if this is a checkpoint image
				inspectReport, err := images.GetImage(ic.ClientCtx, nameOrID, getImageOptions)
				if err != nil {
					return nil, fmt.Errorf("no such container or image: %s", nameOrID)
				}
				checkpointRuntimeName, found := inspectReport.Annotations[define.CheckpointAnnotationRuntimeName]
				if !found {
					return nil, fmt.Errorf("image is not a checkpoint: %s", nameOrID)
				}
				if hostInfo.Host.OCIRuntime.Name != checkpointRuntimeName {
					return nil, fmt.Errorf("container image \"%s\" requires runtime: \"%s\"", nameOrID, checkpointRuntimeName)
				}
				ids = append(ids, inspectReport.ID)
			}
		}
	}
	reports := make([]*entities.RestoreReport, 0, len(ids))
	for _, id := range ids {
		report, err := containers.Restore(ic.ClientCtx, id, options)
		if err != nil {
			reports = append(reports, &entities.RestoreReport{Id: id, Err: err})
		}
		reports = append(reports, report)
	}
	return reports, nil
}

func (ic *ContainerEngine) ContainerCreate(ctx context.Context, s *specgen.SpecGenerator) (*entities.ContainerCreateReport, error) {
	response, err := containers.CreateWithSpec(ic.ClientCtx, s, nil)
	if err != nil {
		return nil, err
	}
	for _, w := range response.Warnings {
		fmt.Fprintf(os.Stderr, "%s\n", w)
	}
	return &entities.ContainerCreateReport{Id: response.ID}, nil
}

func (ic *ContainerEngine) ContainerLogs(_ context.Context, nameOrIDs []string, opts entities.ContainerLogsOptions) error {
	since := opts.Since.Format(time.RFC3339)
	until := opts.Until.Format(time.RFC3339)
	tail := strconv.FormatInt(opts.Tail, 10)
	stdout := opts.StdoutWriter != nil
	stderr := opts.StderrWriter != nil
	options := new(containers.LogOptions).WithFollow(opts.Follow).WithSince(since).WithUntil(until).WithStderr(stderr)
	options.WithStdout(stdout).WithTail(tail)

	var err error
	stdoutCh := make(chan string)
	stderrCh := make(chan string)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		err = containers.Logs(ic.ClientCtx, nameOrIDs[0], options, stdoutCh, stderrCh)
		cancel()
	}()

	for {
		select {
		case <-ctx.Done():
			return err
		case line := <-stdoutCh:
			if opts.StdoutWriter != nil {
				_, _ = io.WriteString(opts.StdoutWriter, line)
			}
		case line := <-stderrCh:
			if opts.StderrWriter != nil {
				_, _ = io.WriteString(opts.StderrWriter, line)
			}
		}
	}
}

func (ic *ContainerEngine) ContainerAttach(ctx context.Context, nameOrID string, opts entities.AttachOptions) error {
	ctrs, err := getContainersByContext(ic.ClientCtx, false, false, []string{nameOrID})
	if err != nil {
		return err
	}
	ctr := ctrs[0]
	if ctr.State != define.ContainerStateRunning.String() {
		return errors.Errorf("you can only attach to running containers")
	}
	options := new(containers.AttachOptions).WithStream(true).WithDetachKeys(opts.DetachKeys)
	return containers.Attach(ic.ClientCtx, nameOrID, opts.Stdin, opts.Stdout, opts.Stderr, nil, options)
}

func makeExecConfig(options entities.ExecOptions) *handlers.ExecCreateConfig {
	env := []string{}
	for k, v := range options.Envs {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	createConfig := new(handlers.ExecCreateConfig)
	createConfig.User = options.User
	createConfig.Privileged = options.Privileged
	createConfig.Tty = options.Tty
	createConfig.AttachStdin = options.Interactive
	createConfig.AttachStdout = true
	createConfig.AttachStderr = true
	createConfig.Detach = false
	createConfig.DetachKeys = options.DetachKeys
	createConfig.Env = env
	createConfig.WorkingDir = options.WorkDir
	createConfig.Cmd = options.Cmd

	return createConfig
}

func (ic *ContainerEngine) ContainerExec(ctx context.Context, nameOrID string, options entities.ExecOptions, streams define.AttachStreams) (int, error) {
	createConfig := makeExecConfig(options)

	sessionID, err := containers.ExecCreate(ic.ClientCtx, nameOrID, createConfig)
	if err != nil {
		return 125, err
	}
	startAndAttachOptions := new(containers.ExecStartAndAttachOptions)
	startAndAttachOptions.WithOutputStream(streams.OutputStream).WithErrorStream(streams.ErrorStream)
	if streams.InputStream != nil {
		startAndAttachOptions.WithInputStream(*streams.InputStream)
	}
	startAndAttachOptions.WithAttachError(streams.AttachError).WithAttachOutput(streams.AttachOutput).WithAttachInput(streams.AttachInput)
	if err := containers.ExecStartAndAttach(ic.ClientCtx, sessionID, startAndAttachOptions); err != nil {
		return 125, err
	}

	inspectOut, err := containers.ExecInspect(ic.ClientCtx, sessionID, nil)
	if err != nil {
		return 125, err
	}

	return inspectOut.ExitCode, nil
}

func (ic *ContainerEngine) ContainerExecDetached(ctx context.Context, nameOrID string, options entities.ExecOptions) (string, error) {
	createConfig := makeExecConfig(options)

	sessionID, err := containers.ExecCreate(ic.ClientCtx, nameOrID, createConfig)
	if err != nil {
		return "", err
	}

	if err := containers.ExecStart(ic.ClientCtx, sessionID, nil); err != nil {
		return "", err
	}

	return sessionID, nil
}

func startAndAttach(ic *ContainerEngine, name string, detachKeys *string, input, output, errput *os.File) error { //nolint
	attachErr := make(chan error)
	attachReady := make(chan bool)
	options := new(containers.AttachOptions).WithStream(true)
	if dk := detachKeys; dk != nil {
		options.WithDetachKeys(*dk)
	}
	go func() {
		err := containers.Attach(ic.ClientCtx, name, input, output, errput, attachReady, options)
		attachErr <- err
	}()
	// Wait for the attach to actually happen before starting
	// the container.
	select {
	case <-attachReady:
		startOptions := new(containers.StartOptions)
		if dk := detachKeys; dk != nil {
			startOptions.WithDetachKeys(*dk)
		}
		if err := containers.Start(ic.ClientCtx, name, startOptions); err != nil {
			return err
		}
	case err := <-attachErr:
		return err
	}
	// If attachReady happens first, wait for containers.Attach to complete
	return <-attachErr
}

func logIfRmError(id string, err error, reports []*reports.RmReport) {
	logError := func(id string, err error) {
		if errorhandling.Contains(err, define.ErrNoSuchCtr) ||
			errorhandling.Contains(err, define.ErrCtrRemoved) ||
			errorhandling.Contains(err, types.ErrLayerUnknown) {
			logrus.Debugf("Container %s does not exist: %v", id, err)
		} else {
			logrus.Errorf("Removing container %s: %v", id, err)
		}
	}
	if err != nil {
		logError(id, err)
	} else {
		for _, report := range reports {
			if report.Err != nil {
				logError(report.Id, report.Err)
			}
		}
	}
}

func (ic *ContainerEngine) ContainerStart(ctx context.Context, namesOrIds []string, options entities.ContainerStartOptions) ([]*entities.ContainerStartReport, error) {
	reports := []*entities.ContainerStartReport{}
	var exitCode = define.ExecErrorCodeGeneric
	containersNamesOrIds := namesOrIds
	all := options.All
	if len(options.Filters) > 0 {
		all = false
		containersNamesOrIds = []string{}
		opts := new(containers.ListOptions).WithFilters(options.Filters).WithAll(true)
		candidates, listErr := containers.List(ic.ClientCtx, opts)
		if listErr != nil {
			return nil, listErr
		}
		for _, candidate := range candidates {
			if options.All {
				containersNamesOrIds = append(containersNamesOrIds, candidate.ID)
				continue
			}
			for _, nameOrID := range namesOrIds {
				if nameOrID == candidate.ID {
					containersNamesOrIds = append(containersNamesOrIds, nameOrID)
					continue
				}
				for _, containerName := range candidate.Names {
					if containerName == nameOrID {
						containersNamesOrIds = append(containersNamesOrIds, nameOrID)
						continue
					}
				}
			}
		}
	}
	ctrs, err := getContainersByContext(ic.ClientCtx, all, false, containersNamesOrIds)
	if err != nil {
		return nil, err
	}
	removeOptions := new(containers.RemoveOptions).WithVolumes(true).WithForce(false)
	removeContainer := func(id string) {
		reports, err := containers.Remove(ic.ClientCtx, id, removeOptions)
		logIfRmError(id, err, reports)
	}

	// There can only be one container if attach was used
	for i, ctr := range ctrs {
		name := ctr.ID
		rawInput := ctr.ID
		if !options.All {
			rawInput = namesOrIds[i]
		}
		report := entities.ContainerStartReport{
			Id:       name,
			RawInput: rawInput,
			ExitCode: exitCode,
		}
		ctrRunning := ctr.State == define.ContainerStateRunning.String()
		if options.Attach {
			err = startAndAttach(ic, name, &options.DetachKeys, options.Stdin, options.Stdout, options.Stderr)
			if err == define.ErrDetach {
				// User manually detached
				// Exit cleanly immediately
				reports = append(reports, &report)
				return reports, nil
			}
			if ctrRunning {
				reports = append(reports, &report)
				return reports, nil
			}

			if err != nil {
				if ctr.AutoRemove {
					removeContainer(ctr.ID)
				}
				report.ExitCode = define.ExitCode(report.Err)
				report.Err = err
				reports = append(reports, &report)
				return reports, errors.Wrapf(report.Err, "unable to start container %s", name)
			}
			if ctr.AutoRemove {
				// Defer the removal, so we can return early if needed and
				// de-spaghetti the code.
				defer func() {
					shouldRestart, err := containers.ShouldRestart(ic.ClientCtx, ctr.ID, nil)
					if err != nil {
						logrus.Errorf("Failed to check if %s should restart: %v", ctr.ID, err)
						return
					}
					logrus.Errorf("Should restart: %v", shouldRestart)

					if !shouldRestart && ctr.AutoRemove {
						removeContainer(ctr.ID)
					}
				}()
			}

			exitCode, err := containers.Wait(ic.ClientCtx, name, nil)
			if err == define.ErrNoSuchCtr {
				// Check events
				event, err := ic.GetLastContainerEvent(ctx, name, events.Exited)
				if err != nil {
					logrus.Errorf("Cannot get exit code: %v", err)
					report.ExitCode = define.ExecErrorCodeNotFound
				} else {
					report.ExitCode = event.ContainerExitCode
				}
			} else {
				report.ExitCode = int(exitCode)
			}
			reports = append(reports, &report)
			return reports, nil
		}
		// Start the container if it's not running already.
		if !ctrRunning {
			err = containers.Start(ic.ClientCtx, name, new(containers.StartOptions).WithDetachKeys(options.DetachKeys))
			if err != nil {
				if ctr.AutoRemove {
					rmOptions := new(containers.RemoveOptions).WithForce(false).WithVolumes(true)
					reports, err := containers.Remove(ic.ClientCtx, ctr.ID, rmOptions)
					logIfRmError(ctr.ID, err, reports)
				}
				report.Err = errors.Wrapf(err, "unable to start container %q", name)
				report.ExitCode = define.ExitCode(err)
				reports = append(reports, &report)
				continue
			}
			report.ExitCode = 0
			reports = append(reports, &report)
		}
	}
	return reports, nil
}

func (ic *ContainerEngine) ContainerList(ctx context.Context, opts entities.ContainerListOptions) ([]entities.ListContainer, error) {
	options := new(containers.ListOptions).WithFilters(opts.Filters).WithAll(opts.All).WithLast(opts.Last)
	options.WithNamespace(opts.Namespace).WithSize(opts.Size).WithSync(opts.Sync).WithExternal(opts.External)
	return containers.List(ic.ClientCtx, options)
}

func (ic *ContainerEngine) ContainerListExternal(ctx context.Context) ([]entities.ListContainer, error) {
	options := new(containers.ListOptions).WithAll(true)
	options.WithNamespace(true).WithSize(true).WithSync(true).WithExternal(true)
	return containers.List(ic.ClientCtx, options)
}

func (ic *ContainerEngine) ContainerRun(ctx context.Context, opts entities.ContainerRunOptions) (*entities.ContainerRunReport, error) {
	con, err := containers.CreateWithSpec(ic.ClientCtx, opts.Spec, nil)
	if err != nil {
		return nil, err
	}
	for _, w := range con.Warnings {
		fmt.Fprintf(os.Stderr, "%s\n", w)
	}
	if opts.CIDFile != "" {
		if err := util.CreateCidFile(opts.CIDFile, con.ID); err != nil {
			return nil, err
		}
	}

	report := entities.ContainerRunReport{Id: con.ID}

	if opts.Detach {
		// Detach and return early
		err := containers.Start(ic.ClientCtx, con.ID, new(containers.StartOptions).WithRecursive(true))
		if err != nil {
			report.ExitCode = define.ExitCode(err)
		}
		return &report, err
	}

	// Attach
	if err := startAndAttach(ic, con.ID, &opts.DetachKeys, opts.InputStream, opts.OutputStream, opts.ErrorStream); err != nil {
		if err == define.ErrDetach {
			return &report, nil
		}

		report.ExitCode = define.ExitCode(err)
		if opts.Rm {
			reports, rmErr := containers.Remove(ic.ClientCtx, con.ID, new(containers.RemoveOptions).WithForce(false).WithVolumes(true))
			if rmErr != nil || reports[0].Err != nil {
				logrus.Debugf("unable to remove container %s after failing to start and attach to it", con.ID)
			}
		}
		return &report, err
	}

	if opts.Rm {
		// Defer the removal, so we can return early if needed and
		// de-spaghetti the code.
		defer func() {
			shouldRestart, err := containers.ShouldRestart(ic.ClientCtx, con.ID, nil)
			if err != nil {
				logrus.Errorf("Failed to check if %s should restart: %v", con.ID, err)
				return
			}

			if !shouldRestart {
				reports, err := containers.Remove(ic.ClientCtx, con.ID, new(containers.RemoveOptions).WithForce(false).WithVolumes(true))
				logIfRmError(con.ID, err, reports)
			}
		}()
	}

	// Wait
	exitCode, waitErr := containers.Wait(ic.ClientCtx, con.ID, nil)
	if waitErr == nil {
		report.ExitCode = int(exitCode)
		return &report, nil
	}

	// Determine why the wait failed.  If the container doesn't exist,
	// consult the events.
	if !errorhandling.Contains(waitErr, define.ErrNoSuchCtr) {
		return &report, waitErr
	}

	// Events
	eventsChannel := make(chan *events.Event)
	eventOptions := entities.EventsOptions{
		EventChan: eventsChannel,
		Filter: []string{
			"type=container",
			fmt.Sprintf("container=%s", con.ID),
			fmt.Sprintf("event=%s", events.Exited),
		},
	}

	var lastEvent *events.Event
	var mutex sync.Mutex
	mutex.Lock()
	// Read the events.
	go func() {
		for e := range eventsChannel {
			lastEvent = e
		}
		mutex.Unlock()
	}()

	eventsErr := ic.Events(ctx, eventOptions)

	// Wait for all events to be read
	mutex.Lock()
	if eventsErr != nil || lastEvent == nil {
		logrus.Errorf("Cannot get exit code: %v", err)
		report.ExitCode = define.ExecErrorCodeNotFound
		return &report, nil // nolint: nilerr
	}

	report.ExitCode = lastEvent.ContainerExitCode
	return &report, err
}

func (ic *ContainerEngine) Diff(ctx context.Context, namesOrIDs []string, opts entities.DiffOptions) (*entities.DiffReport, error) {
	var base string
	options := new(containers.DiffOptions).WithDiffType(opts.Type.String())
	if len(namesOrIDs) > 0 {
		base = namesOrIDs[0]
		if len(namesOrIDs) > 1 {
			options.WithParent(namesOrIDs[1])
		}
	} else {
		return nil, errors.New("no arguments for diff")
	}
	changes, err := containers.Diff(ic.ClientCtx, base, options)
	return &entities.DiffReport{Changes: changes}, err
}

func (ic *ContainerEngine) ContainerCleanup(ctx context.Context, namesOrIds []string, options entities.ContainerCleanupOptions) ([]*entities.ContainerCleanupReport, error) {
	return nil, errors.New("not implemented")
}

func (ic *ContainerEngine) ContainerInit(ctx context.Context, namesOrIds []string, options entities.ContainerInitOptions) ([]*entities.ContainerInitReport, error) {
	ctrs, err := getContainersByContext(ic.ClientCtx, options.All, false, namesOrIds)
	if err != nil {
		return nil, err
	}
	reports := make([]*entities.ContainerInitReport, 0, len(ctrs))
	for _, ctr := range ctrs {
		err := containers.ContainerInit(ic.ClientCtx, ctr.ID, nil)
		// When using all, it is NOT considered an error if a container
		// has already been init'd.
		if err != nil && options.All && strings.Contains(errors.Cause(err).Error(), define.ErrCtrStateInvalid.Error()) {
			err = nil
		}
		reports = append(reports, &entities.ContainerInitReport{
			Err: err,
			Id:  ctr.ID,
		})
	}
	return reports, nil
}

func (ic *ContainerEngine) ContainerMount(ctx context.Context, nameOrIDs []string, options entities.ContainerMountOptions) ([]*entities.ContainerMountReport, error) {
	return nil, errors.New("mounting containers is not supported for remote clients")
}

func (ic *ContainerEngine) ContainerUnmount(ctx context.Context, nameOrIDs []string, options entities.ContainerUnmountOptions) ([]*entities.ContainerUnmountReport, error) {
	return nil, errors.New("unmounting containers is not supported for remote clients")
}

func (ic *ContainerEngine) Config(_ context.Context) (*config.Config, error) {
	return config.Default()
}

func (ic *ContainerEngine) ContainerPort(ctx context.Context, nameOrID string, options entities.ContainerPortOptions) ([]*entities.ContainerPortReport, error) {
	var (
		reports    = []*entities.ContainerPortReport{}
		namesOrIds = []string{}
	)
	if len(nameOrID) > 0 {
		namesOrIds = append(namesOrIds, nameOrID)
	}
	ctrs, err := getContainersByContext(ic.ClientCtx, options.All, false, namesOrIds)
	if err != nil {
		return nil, err
	}
	for _, con := range ctrs {
		if con.State != define.ContainerStateRunning.String() {
			continue
		}
		if len(con.Ports) > 0 {
			reports = append(reports, &entities.ContainerPortReport{
				Id:    con.ID,
				Ports: con.Ports,
			})
		}
	}
	return reports, nil
}

func (ic *ContainerEngine) ContainerCopyFromArchive(ctx context.Context, nameOrID, path string, reader io.Reader, options entities.CopyOptions) (entities.ContainerCopyFunc, error) {
	copyOptions := new(containers.CopyOptions).WithChown(options.Chown).WithRename(options.Rename)
	return containers.CopyFromArchiveWithOptions(ic.ClientCtx, nameOrID, path, reader, copyOptions)
}

func (ic *ContainerEngine) ContainerCopyToArchive(ctx context.Context, nameOrID string, path string, writer io.Writer) (entities.ContainerCopyFunc, error) {
	return containers.CopyToArchive(ic.ClientCtx, nameOrID, path, writer)
}

func (ic *ContainerEngine) ContainerStat(ctx context.Context, nameOrID string, path string) (*entities.ContainerStatReport, error) {
	return containers.Stat(ic.ClientCtx, nameOrID, path)
}

// Shutdown Libpod engine.
func (ic *ContainerEngine) Shutdown(_ context.Context) {
}

func (ic *ContainerEngine) ContainerStats(ctx context.Context, namesOrIds []string, options entities.ContainerStatsOptions) (statsChan chan entities.ContainerStatsReport, err error) {
	if options.Latest {
		return nil, errors.New("latest is not supported for the remote client")
	}
	return containers.Stats(ic.ClientCtx, namesOrIds, new(containers.StatsOptions).WithStream(options.Stream).WithInterval(options.Interval))
}

// ShouldRestart reports back whether the container will restart.
func (ic *ContainerEngine) ShouldRestart(_ context.Context, id string) (bool, error) {
	return containers.ShouldRestart(ic.ClientCtx, id, nil)
}

// ContainerRename renames the given container.
func (ic *ContainerEngine) ContainerRename(ctx context.Context, nameOrID string, opts entities.ContainerRenameOptions) error {
	return containers.Rename(ic.ClientCtx, nameOrID, new(containers.RenameOptions).WithName(opts.NewName))
}

func (ic *ContainerEngine) ContainerClone(ctx context.Context, ctrCloneOpts entities.ContainerCloneOptions) (*entities.ContainerCreateReport, error) {
	return nil, errors.New("cloning a container is not supported on the remote client")
}
