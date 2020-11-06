package abi

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/containers/buildah"
	"github.com/containers/common/pkg/config"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/podman/v2/libpod"
	"github.com/containers/podman/v2/libpod/define"
	"github.com/containers/podman/v2/libpod/events"
	lpfilters "github.com/containers/podman/v2/libpod/filters"
	"github.com/containers/podman/v2/libpod/image"
	"github.com/containers/podman/v2/libpod/logs"
	"github.com/containers/podman/v2/pkg/cgroups"
	"github.com/containers/podman/v2/pkg/checkpoint"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/containers/podman/v2/pkg/domain/infra/abi/terminal"
	parallelctr "github.com/containers/podman/v2/pkg/parallel/ctr"
	"github.com/containers/podman/v2/pkg/ps"
	"github.com/containers/podman/v2/pkg/rootless"
	"github.com/containers/podman/v2/pkg/signal"
	"github.com/containers/podman/v2/pkg/specgen"
	"github.com/containers/podman/v2/pkg/specgen/generate"
	"github.com/containers/podman/v2/pkg/util"
	"github.com/containers/storage"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// getContainersAndInputByContext gets containers whether all, latest, or a slice of names/ids
// is specified.  It also returns a list of the corresponding input name used to lookup each container.
func getContainersAndInputByContext(all, latest bool, names []string, runtime *libpod.Runtime) (ctrs []*libpod.Container, rawInput []string, err error) {
	var ctr *libpod.Container
	ctrs = []*libpod.Container{}

	switch {
	case all:
		ctrs, err = runtime.GetAllContainers()
	case latest:
		ctr, err = runtime.GetLatestContainer()
		if err == nil {
			rawInput = append(rawInput, ctr.ID())
			ctrs = append(ctrs, ctr)
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
	return
}

// getContainersByContext gets containers whether all, latest, or a slice of names/ids
// is specified.
func getContainersByContext(all, latest bool, names []string, runtime *libpod.Runtime) (ctrs []*libpod.Container, err error) {
	ctrs, _, err = getContainersAndInputByContext(all, latest, names, runtime)
	return
}

// ContainerExists returns whether the container exists in container storage
func (ic *ContainerEngine) ContainerExists(ctx context.Context, nameOrID string, options entities.ContainerExistsOptions) (*entities.BoolReport, error) {
	_, err := ic.Libpod.LookupContainer(nameOrID)
	if err != nil {
		if errors.Cause(err) != define.ErrNoSuchCtr {
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
	ctrs, err := getContainersByContext(false, options.Latest, namesOrIds, ic.Libpod)
	if err != nil {
		return nil, err
	}
	responses := make([]entities.WaitReport, 0, len(ctrs))
	for _, c := range ctrs {
		response := entities.WaitReport{Id: c.ID()}
		exitCode, err := c.WaitForConditionWithInterval(options.Interval, options.Condition)
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
	var (
		err error
	)
	ctrs := []*libpod.Container{} //nolint
	if options.All {
		ctrs, err = ic.Libpod.GetAllContainers()
	} else {
		ctrs, err = getContainersByContext(false, false, namesOrIds, ic.Libpod)
	}
	if err != nil {
		return nil, err
	}
	report := make([]*entities.PauseUnpauseReport, 0, len(ctrs))
	for _, c := range ctrs {
		err := c.Pause()
		report = append(report, &entities.PauseUnpauseReport{Id: c.ID(), Err: err})
	}
	return report, nil
}

func (ic *ContainerEngine) ContainerUnpause(ctx context.Context, namesOrIds []string, options entities.PauseUnPauseOptions) ([]*entities.PauseUnpauseReport, error) {
	var (
		err error
	)
	ctrs := []*libpod.Container{} //nolint
	if options.All {
		ctrs, err = ic.Libpod.GetAllContainers()
	} else {
		ctrs, err = getContainersByContext(false, false, namesOrIds, ic.Libpod)
	}
	if err != nil {
		return nil, err
	}
	report := make([]*entities.PauseUnpauseReport, 0, len(ctrs))
	for _, c := range ctrs {
		err := c.Unpause()
		report = append(report, &entities.PauseUnpauseReport{Id: c.ID(), Err: err})
	}
	return report, nil
}
func (ic *ContainerEngine) ContainerStop(ctx context.Context, namesOrIds []string, options entities.StopOptions) ([]*entities.StopReport, error) {
	names := namesOrIds
	for _, cidFile := range options.CIDFiles {
		content, err := ioutil.ReadFile(cidFile)
		if err != nil {
			return nil, errors.Wrap(err, "error reading CIDFile")
		}
		id := strings.Split(string(content), "\n")[0]
		names = append(names, id)
	}
	ctrs, err := getContainersByContext(options.All, options.Latest, names, ic.Libpod)
	if err != nil && !(options.Ignore && errors.Cause(err) == define.ErrNoSuchCtr) {
		return nil, err
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
			case errors.Cause(err) == define.ErrCtrStopped:
				logrus.Debugf("Container %s is already stopped", c.ID())
			case options.All && errors.Cause(err) == define.ErrCtrStateInvalid:
				logrus.Debugf("Container %s is not running, could not stop", c.ID())
			default:
				return err
			}
		}
		if c.AutoRemove() {
			// Issue #7384: if the container is configured for
			// auto-removal, it might already have been removed at
			// this point.
			return nil
		}
		return c.Cleanup(ctx)
	})
	if err != nil {
		return nil, err
	}
	reports := make([]*entities.StopReport, 0, len(errMap))
	for ctr, err := range errMap {
		report := new(entities.StopReport)
		report.Id = ctr.ID()
		report.Err = err
		reports = append(reports, report)
	}
	return reports, nil
}

func (ic *ContainerEngine) ContainerPrune(ctx context.Context, options entities.ContainerPruneOptions) (*entities.ContainerPruneReport, error) {
	filterFuncs := make([]libpod.ContainerFilter, 0, len(options.Filters))
	for k, v := range options.Filters {
		generatedFunc, err := lpfilters.GenerateContainerFilterFuncs(k, v, ic.Libpod)
		if err != nil {
			return nil, err
		}
		filterFuncs = append(filterFuncs, generatedFunc)
	}
	return ic.pruneContainersHelper(filterFuncs)
}

func (ic *ContainerEngine) pruneContainersHelper(filterFuncs []libpod.ContainerFilter) (*entities.ContainerPruneReport, error) {
	prunedContainers, pruneErrors, err := ic.Libpod.PruneContainers(filterFuncs)
	if err != nil {
		return nil, err
	}
	report := entities.ContainerPruneReport{
		ID:  prunedContainers,
		Err: pruneErrors,
	}
	return &report, nil
}

func (ic *ContainerEngine) ContainerKill(ctx context.Context, namesOrIds []string, options entities.KillOptions) ([]*entities.KillReport, error) {
	sig, err := signal.ParseSignalNameOrNumber(options.Signal)
	if err != nil {
		return nil, err
	}
	ctrs, err := getContainersByContext(options.All, options.Latest, namesOrIds, ic.Libpod)
	if err != nil {
		return nil, err
	}
	reports := make([]*entities.KillReport, 0, len(ctrs))
	for _, con := range ctrs {
		reports = append(reports, &entities.KillReport{
			Id:  con.ID(),
			Err: con.Kill(uint(sig)),
		})
	}
	return reports, nil
}
func (ic *ContainerEngine) ContainerRestart(ctx context.Context, namesOrIds []string, options entities.RestartOptions) ([]*entities.RestartReport, error) {
	var (
		ctrs []*libpod.Container
		err  error
	)

	if options.Running {
		ctrs, err = ic.Libpod.GetRunningContainers()
		if err != nil {
			return nil, err
		}
	} else {
		ctrs, err = getContainersByContext(options.All, options.Latest, namesOrIds, ic.Libpod)
		if err != nil {
			return nil, err
		}
	}

	reports := make([]*entities.RestartReport, 0, len(ctrs))
	for _, con := range ctrs {
		timeout := con.StopTimeout()
		if options.Timeout != nil {
			timeout = *options.Timeout
		}
		reports = append(reports, &entities.RestartReport{
			Id:  con.ID(),
			Err: con.RestartWithTimeout(ctx, timeout),
		})
	}
	return reports, nil
}

func (ic *ContainerEngine) ContainerRm(ctx context.Context, namesOrIds []string, options entities.RmOptions) ([]*entities.RmReport, error) {
	reports := []*entities.RmReport{}

	names := namesOrIds
	for _, cidFile := range options.CIDFiles {
		content, err := ioutil.ReadFile(cidFile)
		if err != nil {
			return nil, errors.Wrap(err, "error reading CIDFile")
		}
		id := strings.Split(string(content), "\n")[0]
		names = append(names, id)
	}

	// Attempt to remove named containers directly from storage, if container is defined in libpod
	// this will fail and code will fall through to removing the container from libpod.`
	tmpNames := []string{}
	for _, ctr := range names {
		report := entities.RmReport{Id: ctr}
		if err := ic.Libpod.RemoveStorageContainer(ctr, options.Force); err != nil {
			// remove container names that we successfully deleted
			tmpNames = append(tmpNames, ctr)
		} else {
			reports = append(reports, &report)
		}
	}
	if len(tmpNames) < len(names) {
		names = tmpNames
	}

	ctrs, err := getContainersByContext(options.All, options.Latest, names, ic.Libpod)
	if err != nil && !(options.Ignore && errors.Cause(err) == define.ErrNoSuchCtr) {
		// Failed to get containers. If force is specified, get the containers ID
		// and evict them
		if !options.Force {
			return nil, err
		}

		for _, ctr := range names {
			logrus.Debugf("Evicting container %q", ctr)
			report := entities.RmReport{Id: ctr}
			id, err := ic.Libpod.EvictContainer(ctx, ctr, options.Volumes)
			if err != nil {
				if options.Ignore && errors.Cause(err) == define.ErrNoSuchCtr {
					logrus.Debugf("Ignoring error (--allow-missing): %v", err)
					reports = append(reports, &report)
					continue
				}
				report.Err = errors.Wrapf(err, "failed to evict container: %q", id)
				reports = append(reports, &report)
				continue
			}
			reports = append(reports, &report)
		}
		return reports, nil
	}

	errMap, err := parallelctr.ContainerOp(ctx, ctrs, func(c *libpod.Container) error {
		err := ic.Libpod.RemoveContainer(ctx, c, options.Force, options.Volumes)
		if err != nil {
			if options.Ignore && errors.Cause(err) == define.ErrNoSuchCtr {
				logrus.Debugf("Ignoring error (--allow-missing): %v", err)
				return nil
			}
			logrus.Debugf("Failed to remove container %s: %s", c.ID(), err.Error())
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	for ctr, err := range errMap {
		report := new(entities.RmReport)
		report.Id = ctr.ID()
		report.Err = err
		reports = append(reports, report)
	}
	return reports, nil
}

func (ic *ContainerEngine) ContainerInspect(ctx context.Context, namesOrIds []string, options entities.InspectOptions) ([]*entities.ContainerInspectReport, []error, error) {
	if options.Latest {
		ctr, err := ic.Libpod.GetLatestContainer()
		if err != nil {
			if errors.Cause(err) == define.ErrNoSuchCtr {
				return nil, []error{errors.Wrapf(err, "no containers to inspect")}, nil
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
			if errors.Cause(err) == define.ErrNoSuchCtr {
				errs = append(errs, errors.Errorf("no such container %s", name))
				continue
			}
			return nil, nil, err
		}

		inspect, err := ctr.Inspect(options.Size)
		if err != nil {
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
		return nil, errors.Wrap(err, "unable to lookup requested container")
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
			return nil, errors.Errorf("messages are only compatible with the docker image format (-f docker)")
		}
	case "docker":
		mimeType = manifest.DockerV2Schema2MediaType
	default:
		return nil, errors.Errorf("unrecognized image format %q", options.Format)
	}
	sc := image.GetSystemContext(rtc.Engine.SignaturePolicyPath, "", false)
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
		err  error
		cons []*libpod.Container
	)
	checkOpts := libpod.ContainerCheckpointOptions{
		Keep:           options.Keep,
		TCPEstablished: options.TCPEstablished,
		TargetFile:     options.Export,
		IgnoreRootfs:   options.IgnoreRootFS,
		KeepRunning:    options.LeaveRunning,
	}

	if options.All {
		running := func(c *libpod.Container) bool {
			state, _ := c.State()
			return state == define.ContainerStateRunning
		}
		cons, err = ic.Libpod.GetContainers(running)
	} else {
		cons, err = getContainersByContext(false, options.Latest, namesOrIds, ic.Libpod)
	}
	if err != nil {
		return nil, err
	}
	reports := make([]*entities.CheckpointReport, 0, len(cons))
	for _, con := range cons {
		err = con.Checkpoint(ctx, checkOpts)
		reports = append(reports, &entities.CheckpointReport{
			Err: err,
			Id:  con.ID(),
		})
	}
	return reports, nil
}

func (ic *ContainerEngine) ContainerRestore(ctx context.Context, namesOrIds []string, options entities.RestoreOptions) ([]*entities.RestoreReport, error) {
	var (
		cons []*libpod.Container
		err  error
	)

	restoreOptions := libpod.ContainerCheckpointOptions{
		Keep:            options.Keep,
		TCPEstablished:  options.TCPEstablished,
		TargetFile:      options.Import,
		Name:            options.Name,
		IgnoreRootfs:    options.IgnoreRootFS,
		IgnoreStaticIP:  options.IgnoreStaticIP,
		IgnoreStaticMAC: options.IgnoreStaticMAC,
	}

	filterFuncs := []libpod.ContainerFilter{
		func(c *libpod.Container) bool {
			state, _ := c.State()
			return state == define.ContainerStateExited
		},
	}

	switch {
	case options.Import != "":
		cons, err = checkpoint.CRImportCheckpoint(ctx, ic.Libpod, options.Import, options.Name)
	case options.All:
		cons, err = ic.Libpod.GetContainers(filterFuncs...)
	default:
		cons, err = getContainersByContext(false, options.Latest, namesOrIds, ic.Libpod)
	}
	if err != nil {
		return nil, err
	}
	reports := make([]*entities.RestoreReport, 0, len(cons))
	for _, con := range cons {
		err := con.Restore(ctx, restoreOptions)
		reports = append(reports, &entities.RestoreReport{
			Err: err,
			Id:  con.ID(),
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
	ctr, err := generate.MakeContainer(ctx, ic.Libpod, s)
	if err != nil {
		return nil, err
	}
	return &entities.ContainerCreateReport{Id: ctr.ID()}, nil
}

func (ic *ContainerEngine) ContainerAttach(ctx context.Context, nameOrID string, options entities.AttachOptions) error {
	ctrs, err := getContainersByContext(false, options.Latest, []string{nameOrID}, ic.Libpod)
	if err != nil {
		return err
	}
	ctr := ctrs[0]
	conState, err := ctr.State()
	if err != nil {
		return errors.Wrapf(err, "unable to determine state of %s", ctr.ID())
	}
	if conState != define.ContainerStateRunning {
		return errors.Errorf("you can only attach to running containers")
	}

	// If the container is in a pod, also set to recursively start dependencies
	err = terminal.StartAttachCtr(ctx, ctr, options.Stdout, options.Stderr, options.Stdin, options.DetachKeys, options.SigProxy, false, ctr.PodID() != "")
	if err != nil && errors.Cause(err) != define.ErrDetach {
		return errors.Wrapf(err, "error attaching to container %s", ctr.ID())
	}
	os.Stdout.WriteString("\n")
	return nil
}

func makeExecConfig(options entities.ExecOptions) *libpod.ExecConfig {
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

	return execConfig
}

func checkExecPreserveFDs(options entities.ExecOptions) error {
	if options.PreserveFDs > 0 {
		entries, err := ioutil.ReadDir("/proc/self/fd")
		if err != nil {
			return err
		}

		m := make(map[int]bool)
		for _, e := range entries {
			i, err := strconv.Atoi(e.Name())
			if err != nil {
				return errors.Wrapf(err, "cannot parse %s in /proc/self/fd", e.Name())
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
	ctrs, err := getContainersByContext(false, options.Latest, []string{nameOrID}, ic.Libpod)
	if err != nil {
		return ec, err
	}
	ctr := ctrs[0]

	execConfig := makeExecConfig(options)

	ec, err = terminal.ExecAttachCtr(ctx, ctr, execConfig, &streams)
	return define.TranslateExecErrorToExitCode(ec, err), err
}

func (ic *ContainerEngine) ContainerExecDetached(ctx context.Context, nameOrID string, options entities.ExecOptions) (string, error) {
	err := checkExecPreserveFDs(options)
	if err != nil {
		return "", err
	}
	ctrs, err := getContainersByContext(false, options.Latest, []string{nameOrID}, ic.Libpod)
	if err != nil {
		return "", err
	}
	ctr := ctrs[0]

	execConfig := makeExecConfig(options)

	// Make an exit command
	storageConfig := ic.Libpod.StorageConfig()
	runtimeConfig, err := ic.Libpod.GetConfig()
	if err != nil {
		return "", errors.Wrapf(err, "error retrieving Libpod configuration to build exec exit command")
	}
	// TODO: Add some ability to toggle syslog
	exitCommandArgs, err := generate.CreateExitCommandArgs(storageConfig, runtimeConfig, false, true, true)
	if err != nil {
		return "", errors.Wrapf(err, "error constructing exit command for exec session")
	}
	execConfig.ExitCommand = exitCommandArgs

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
	ctrs, rawInputs, err := getContainersAndInputByContext(false, options.Latest, namesOrIds, ic.Libpod)
	if err != nil {
		return nil, err
	}
	// There can only be one container if attach was used
	for i := range ctrs {
		ctr := ctrs[i]
		rawInput := rawInputs[i]
		ctrState, err := ctr.State()
		if err != nil {
			return nil, err
		}
		ctrRunning := ctrState == define.ContainerStateRunning

		if options.Attach {
			err = terminal.StartAttachCtr(ctx, ctr, options.Stdout, options.Stderr, options.Stdin, options.DetachKeys, options.SigProxy, !ctrRunning, ctr.PodID() != "")
			if errors.Cause(err) == define.ErrDetach {
				// User manually detached
				// Exit cleanly immediately
				reports = append(reports, &entities.ContainerStartReport{
					Id:       ctr.ID(),
					RawInput: rawInput,
					Err:      nil,
					ExitCode: 0,
				})
				return reports, nil
			}

			if errors.Cause(err) == define.ErrWillDeadlock {
				logrus.Debugf("Deadlock error: %v", err)
				reports = append(reports, &entities.ContainerStartReport{
					Id:       ctr.ID(),
					RawInput: rawInput,
					Err:      err,
					ExitCode: define.ExitCode(err),
				})
				return reports, errors.Errorf("attempting to start container %s would cause a deadlock; please run 'podman system renumber' to resolve", ctr.ID())
			}

			if ctrRunning {
				reports = append(reports, &entities.ContainerStartReport{
					Id:       ctr.ID(),
					RawInput: rawInput,
					Err:      nil,
					ExitCode: 0,
				})
				return reports, err
			}

			if err != nil {
				reports = append(reports, &entities.ContainerStartReport{
					Id:       ctr.ID(),
					RawInput: rawInput,
					Err:      err,
					ExitCode: exitCode,
				})
				return reports, errors.Wrapf(err, "unable to start container %s", ctr.ID())
			}

			if ecode, err := ctr.Wait(); err != nil {
				if errors.Cause(err) == define.ErrNoSuchCtr {
					// Check events
					event, err := ic.Libpod.GetLastContainerEvent(ctx, ctr.ID(), events.Exited)
					if err != nil {
						logrus.Errorf("Cannot get exit code: %v", err)
						exitCode = define.ExecErrorCodeNotFound
					} else {
						exitCode = event.ContainerExitCode
					}
				}
			} else {
				exitCode = int(ecode)
			}
			reports = append(reports, &entities.ContainerStartReport{
				Id:       ctr.ID(),
				RawInput: rawInput,
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
				RawInput: rawInput,
				ExitCode: 125,
			}
			if err := ctr.Start(ctx, ctr.PodID() != ""); err != nil {
				// if lastError != nil {
				//	fmt.Fprintln(os.Stderr, lastError)
				// }
				report.Err = err
				if errors.Cause(err) == define.ErrWillDeadlock {
					report.Err = errors.Wrapf(err, "please run 'podman system renumber' to resolve deadlocks")
					reports = append(reports, report)
					continue
				}
				report.Err = errors.Wrapf(err, "unable to start container %q", ctr.ID())
				reports = append(reports, report)
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

// ContainerDiff provides changes to given container
func (ic *ContainerEngine) ContainerDiff(ctx context.Context, nameOrID string, opts entities.DiffOptions) (*entities.DiffReport, error) {
	if opts.Latest {
		ctnr, err := ic.Libpod.GetLatestContainer()
		if err != nil {
			return nil, errors.Wrap(err, "unable to get latest container")
		}
		nameOrID = ctnr.ID()
	}
	changes, err := ic.Libpod.GetDiff("", nameOrID)
	return &entities.DiffReport{Changes: changes}, err
}

func (ic *ContainerEngine) ContainerRun(ctx context.Context, opts entities.ContainerRunOptions) (*entities.ContainerRunReport, error) {
	warn, err := generate.CompleteSpec(ctx, ic.Libpod, opts.Spec)
	if err != nil {
		return nil, err
	}
	// Print warnings
	for _, w := range warn {
		fmt.Fprintf(os.Stderr, "%s\n", w)
	}
	ctr, err := generate.MakeContainer(ctx, ic.Libpod, opts.Spec)
	if err != nil {
		return nil, err
	}

	if opts.CIDFile != "" {
		if err := util.CreateCidFile(opts.CIDFile, ctr.ID()); err != nil {
			return nil, err
		}
	}

	var joinPod bool
	if len(ctr.PodID()) > 0 {
		joinPod = true
	}
	report := entities.ContainerRunReport{Id: ctr.ID()}

	if logrus.GetLevel() == logrus.DebugLevel {
		cgroupPath, err := ctr.CGroupPath()
		if err == nil {
			logrus.Debugf("container %q has CgroupParent %q", ctr.ID(), cgroupPath)
		}
	}
	if opts.Detach {
		// if the container was created as part of a pod, also start its dependencies, if any.
		if err := ctr.Start(ctx, joinPod); err != nil {
			// This means the command did not exist
			report.ExitCode = define.ExitCode(err)
			return &report, err
		}

		return &report, nil
	}

	// if the container was created as part of a pod, also start its dependencies, if any.
	if err := terminal.StartAttachCtr(ctx, ctr, opts.OutputStream, opts.ErrorStream, opts.InputStream, opts.DetachKeys, opts.SigProxy, true, joinPod); err != nil {
		// We've manually detached from the container
		// Do not perform cleanup, or wait for container exit code
		// Just exit immediately
		if errors.Cause(err) == define.ErrDetach {
			report.ExitCode = 0
			return &report, nil
		}
		if opts.Rm {
			if deleteError := ic.Libpod.RemoveContainer(ctx, ctr, true, false); deleteError != nil {
				logrus.Debugf("unable to remove container %s after failing to start and attach to it", ctr.ID())
			}
		}
		if errors.Cause(err) == define.ErrWillDeadlock {
			logrus.Debugf("Deadlock error on %q: %v", ctr.ID(), err)
			report.ExitCode = define.ExitCode(err)
			return &report, errors.Errorf("attempting to start container %s would cause a deadlock; please run 'podman system renumber' to resolve", ctr.ID())
		}
		report.ExitCode = define.ExitCode(err)
		return &report, err
	}

	if ecode, err := ctr.Wait(); err != nil {
		if errors.Cause(err) == define.ErrNoSuchCtr {
			// Check events
			event, err := ic.Libpod.GetLastContainerEvent(ctx, ctr.ID(), events.Exited)
			if err != nil {
				logrus.Errorf("Cannot get exit code: %v", err)
				report.ExitCode = define.ExecErrorCodeNotFound
			} else {
				report.ExitCode = event.ContainerExitCode
			}
		}
	} else {
		report.ExitCode = int(ecode)
	}
	if opts.Rm && !ctr.ShouldRestart(ctx) {
		if err := ic.Libpod.RemoveContainer(ctx, ctr, false, true); err != nil {
			if errors.Cause(err) == define.ErrNoSuchCtr ||
				errors.Cause(err) == define.ErrCtrRemoved {
				logrus.Warnf("Container %s does not exist: %v", ctr.ID(), err)
			} else {
				logrus.Errorf("Error removing container %s: %v", ctr.ID(), err)
			}
		}
	}
	return &report, nil
}

func (ic *ContainerEngine) ContainerLogs(ctx context.Context, containers []string, options entities.ContainerLogsOptions) error {
	if options.Writer == nil {
		return errors.New("no io.Writer set for container logs")
	}

	var wg sync.WaitGroup

	ctrs, err := getContainersByContext(false, options.Latest, containers, ic.Libpod)
	if err != nil {
		return err
	}

	logOpts := &logs.LogOptions{
		Multi:      len(ctrs) > 1,
		Details:    options.Details,
		Follow:     options.Follow,
		Since:      options.Since,
		Tail:       options.Tail,
		Timestamps: options.Timestamps,
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
		fmt.Fprintln(options.Writer, line.String(logOpts))
	}

	return nil
}

func (ic *ContainerEngine) ContainerCleanup(ctx context.Context, namesOrIds []string, options entities.ContainerCleanupOptions) ([]*entities.ContainerCleanupReport, error) {
	reports := []*entities.ContainerCleanupReport{}
	ctrs, err := getContainersByContext(options.All, options.Latest, namesOrIds, ic.Libpod)
	if err != nil {
		return nil, err
	}
	for _, ctr := range ctrs {
		var err error
		report := entities.ContainerCleanupReport{Id: ctr.ID()}

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
			err = ic.Libpod.RemoveContainer(ctx, ctr, false, true)
			if err != nil {
				report.RmErr = errors.Wrapf(err, "failed to cleanup and remove container %v", ctr.ID())
			}
		} else {
			err := ctr.Cleanup(ctx)
			if err != nil {
				report.CleanErr = errors.Wrapf(err, "failed to cleanup container %v", ctr.ID())
			}
		}

		if options.RemoveImage {
			_, imageName := ctr.Image()
			ctrImage, err := ic.Libpod.ImageRuntime().NewFromLocal(imageName)
			if err != nil {
				report.RmiErr = err
				reports = append(reports, &report)
				continue
			}
			_, err = ic.Libpod.RemoveImage(ctx, ctrImage, false)
			report.RmiErr = err
		}

		reports = append(reports, &report)
	}
	return reports, nil
}

func (ic *ContainerEngine) ContainerInit(ctx context.Context, namesOrIds []string, options entities.ContainerInitOptions) ([]*entities.ContainerInitReport, error) {
	ctrs, err := getContainersByContext(options.All, options.Latest, namesOrIds, ic.Libpod)
	if err != nil {
		return nil, err
	}
	reports := make([]*entities.ContainerInitReport, 0, len(ctrs))
	for _, ctr := range ctrs {
		report := entities.ContainerInitReport{Id: ctr.ID()}
		err := ctr.Init(ctx, ctr.PodID() != "")

		// If we're initializing all containers, ignore invalid state errors
		if options.All && errors.Cause(err) == define.ErrCtrStateInvalid {
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

	ctrs, err := getContainersByContext(options.All, options.Latest, names, ic.Libpod)
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
	ctrs, err = getContainersByContext(true, false, []string{}, ic.Libpod)
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
					if errors.Cause(report.Err) != define.ErrCtrExists {
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
	ctrs, err := getContainersByContext(options.All, options.Latest, names, ic.Libpod)
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
			if options.All && errors.Cause(err) == storage.ErrLayerNotMounted {
				logrus.Debugf("Error umounting container %s, storage.ErrLayerNotMounted", ctr.ID())
				continue
			}
			report.Err = errors.Wrapf(err, "error unmounting container %s", ctr.ID())
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
	ctrs, err := getContainersByContext(options.All, options.Latest, []string{nameOrID}, ic.Libpod)
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
				return nil, errors.Wrapf(err, "unable to get list of containers")
			}

			reportStats := []define.ContainerStats{}
			for _, ctr := range containers {
				prev, ok := containerStats[ctr.ID()]
				if !ok {
					prev = &define.ContainerStats{}
				}

				stats, err := ctr.GetContainerStats(prev)
				if err != nil {
					cause := errors.Cause(err)
					if queryAll && (cause == define.ErrCtrRemoved || cause == define.ErrNoSuchCtr || cause == define.ErrCtrStateInvalid) {
						continue
					}
					if cause == cgroups.ErrCgroupV1Rootless {
						err = cause
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

		time.Sleep(time.Second)
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
