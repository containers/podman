// +build ABISupport

package abi

import (
	"context"
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"
	"sync"

	"github.com/containers/buildah"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/libpod/events"
	"github.com/containers/libpod/libpod/image"
	"github.com/containers/libpod/libpod/logs"
	"github.com/containers/libpod/pkg/checkpoint"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/containers/libpod/pkg/domain/infra/abi/terminal"
	"github.com/containers/libpod/pkg/ps"
	"github.com/containers/libpod/pkg/signal"
	"github.com/containers/libpod/pkg/specgen"
	"github.com/containers/libpod/pkg/specgen/generate"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// getContainersByContext gets pods whether all, latest, or a slice of names/ids
// is specified.
func getContainersByContext(all, latest bool, names []string, runtime *libpod.Runtime) (ctrs []*libpod.Container, err error) {
	var ctr *libpod.Container
	ctrs = []*libpod.Container{}

	switch {
	case all:
		ctrs, err = runtime.GetAllContainers()
	case latest:
		ctr, err = runtime.GetLatestContainer()
		ctrs = append(ctrs, ctr)
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
				ctrs = append(ctrs, ctr)
			}
		}
	}
	return
}

// TODO: Should return *entities.ContainerExistsReport, error
func (ic *ContainerEngine) ContainerExists(ctx context.Context, nameOrId string) (*entities.BoolReport, error) {
	_, err := ic.Libpod.LookupContainer(nameOrId)
	if err != nil && errors.Cause(err) != define.ErrNoSuchCtr {
		return nil, err
	}
	return &entities.BoolReport{Value: err == nil}, nil
}

func (ic *ContainerEngine) ContainerWait(ctx context.Context, namesOrIds []string, options entities.WaitOptions) ([]entities.WaitReport, error) {
	var (
		responses []entities.WaitReport
	)
	ctrs, err := getContainersByContext(false, options.Latest, namesOrIds, ic.Libpod)
	if err != nil {
		return nil, err
	}
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
		ctrs   []*libpod.Container
		err    error
		report []*entities.PauseUnpauseReport
	)
	if options.All {
		ctrs, err = ic.Libpod.GetAllContainers()
	} else {
		ctrs, err = getContainersByContext(false, false, namesOrIds, ic.Libpod)
	}
	if err != nil {
		return nil, err
	}
	for _, c := range ctrs {
		err := c.Pause()
		report = append(report, &entities.PauseUnpauseReport{Id: c.ID(), Err: err})
	}
	return report, nil
}

func (ic *ContainerEngine) ContainerUnpause(ctx context.Context, namesOrIds []string, options entities.PauseUnPauseOptions) ([]*entities.PauseUnpauseReport, error) {
	var (
		ctrs   []*libpod.Container
		err    error
		report []*entities.PauseUnpauseReport
	)
	if options.All {
		ctrs, err = ic.Libpod.GetAllContainers()
	} else {
		ctrs, err = getContainersByContext(false, false, namesOrIds, ic.Libpod)
	}
	if err != nil {
		return nil, err
	}
	for _, c := range ctrs {
		err := c.Unpause()
		report = append(report, &entities.PauseUnpauseReport{Id: c.ID(), Err: err})
	}
	return report, nil
}
func (ic *ContainerEngine) ContainerStop(ctx context.Context, namesOrIds []string, options entities.StopOptions) ([]*entities.StopReport, error) {
	var (
		reports []*entities.StopReport
	)
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
	for _, con := range ctrs {
		report := entities.StopReport{Id: con.ID()}
		err = con.StopWithTimeout(options.Timeout)
		if err != nil {
			// These first two are considered non-fatal under the right conditions
			if errors.Cause(err) == define.ErrCtrStopped {
				logrus.Debugf("Container %s is already stopped", con.ID())
				reports = append(reports, &report)
				continue

			} else if options.All && errors.Cause(err) == define.ErrCtrStateInvalid {
				logrus.Debugf("Container %s is not running, could not stop", con.ID())
				reports = append(reports, &report)
				continue
			}
			report.Err = err
			reports = append(reports, &report)
			continue
		}
		reports = append(reports, &report)
	}
	return reports, nil
}

func (ic *ContainerEngine) ContainerKill(ctx context.Context, namesOrIds []string, options entities.KillOptions) ([]*entities.KillReport, error) {
	var (
		reports []*entities.KillReport
	)
	sig, err := signal.ParseSignalNameOrNumber(options.Signal)
	if err != nil {
		return nil, err
	}
	ctrs, err := getContainersByContext(options.All, options.Latest, namesOrIds, ic.Libpod)
	if err != nil {
		return nil, err
	}
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
		reports []*entities.RestartReport
	)
	ctrs, err := getContainersByContext(options.All, options.Latest, namesOrIds, ic.Libpod)
	if err != nil {
		return nil, err
	}
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
	var (
		reports []*entities.RmReport
	)
	if options.Storage {
		for _, ctr := range namesOrIds {
			report := entities.RmReport{Id: ctr}
			if err := ic.Libpod.RemoveStorageContainer(ctr, options.Force); err != nil {
				report.Err = err
			}
			reports = append(reports, &report)
		}
		return reports, nil
	}

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
		// Failed to get containers. If force is specified, get the containers ID
		// and evict them
		if !options.Force {
			return nil, err
		}

		for _, ctr := range namesOrIds {
			logrus.Debugf("Evicting container %q", ctr)
			report := entities.RmReport{Id: ctr}
			id, err := ic.Libpod.EvictContainer(ctx, ctr, options.Volumes)
			if err != nil {
				if options.Ignore && errors.Cause(err) == define.ErrNoSuchCtr {
					logrus.Debugf("Ignoring error (--allow-missing): %v", err)
					reports = append(reports, &report)
					continue
				}
				report.Err = errors.Wrapf(err, "Failed to evict container: %q", id)
				reports = append(reports, &report)
				continue
			}
			reports = append(reports, &report)
		}
		return reports, nil
	}

	for _, c := range ctrs {
		report := entities.RmReport{Id: c.ID()}
		err := ic.Libpod.RemoveContainer(ctx, c, options.Force, options.Volumes)
		if err != nil {
			if options.Ignore && errors.Cause(err) == define.ErrNoSuchCtr {
				logrus.Debugf("Ignoring error (--allow-missing): %v", err)
				reports = append(reports, &report)
				continue
			}
			logrus.Debugf("Failed to remove container %s: %s", c.ID(), err.Error())
			report.Err = err
			reports = append(reports, &report)
			continue
		}
		reports = append(reports, &report)
	}
	return reports, nil
}

func (ic *ContainerEngine) ContainerInspect(ctx context.Context, namesOrIds []string, options entities.InspectOptions) ([]*entities.ContainerInspectReport, error) {
	var reports []*entities.ContainerInspectReport
	ctrs, err := getContainersByContext(false, options.Latest, namesOrIds, ic.Libpod)
	if err != nil {
		return nil, err
	}
	for _, c := range ctrs {
		data, err := c.Inspect(options.Size)
		if err != nil {
			return nil, err
		}
		reports = append(reports, &entities.ContainerInspectReport{InspectContainerData: data})
	}
	return reports, nil
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

func (ic *ContainerEngine) ContainerCommit(ctx context.Context, nameOrId string, options entities.CommitOptions) (*entities.CommitReport, error) {
	var (
		mimeType string
	)
	ctr, err := ic.Libpod.LookupContainer(nameOrId)
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

func (ic *ContainerEngine) ContainerExport(ctx context.Context, nameOrId string, options entities.ContainerExportOptions) error {
	ctr, err := ic.Libpod.LookupContainer(nameOrId)
	if err != nil {
		return err
	}
	return ctr.Export(options.Output)
}

func (ic *ContainerEngine) ContainerCheckpoint(ctx context.Context, namesOrIds []string, options entities.CheckpointOptions) ([]*entities.CheckpointReport, error) {
	var (
		err     error
		cons    []*libpod.Container
		reports []*entities.CheckpointReport
	)
	checkOpts := libpod.ContainerCheckpointOptions{
		Keep:           options.Keep,
		TCPEstablished: options.TCPEstablished,
		TargetFile:     options.Export,
		IgnoreRootfs:   options.IgnoreRootFS,
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
		cons        []*libpod.Container
		err         error
		filterFuncs []libpod.ContainerFilter
		reports     []*entities.RestoreReport
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

	filterFuncs = append(filterFuncs, func(c *libpod.Container) bool {
		state, _ := c.State()
		return state == define.ContainerStateExited
	})

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
	if err := generate.CompleteSpec(ctx, ic.Libpod, s); err != nil {
		return nil, err
	}
	ctr, err := generate.MakeContainer(ic.Libpod, s)
	if err != nil {
		return nil, err
	}
	return &entities.ContainerCreateReport{Id: ctr.ID()}, nil
}

func (ic *ContainerEngine) ContainerAttach(ctx context.Context, nameOrId string, options entities.AttachOptions) error {
	ctrs, err := getContainersByContext(false, options.Latest, []string{nameOrId}, ic.Libpod)
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
	if err := terminal.StartAttachCtr(ctx, ctr, options.Stdin, options.Stderr, options.Stdin, options.DetachKeys, options.SigProxy, false, ctr.PodID() != ""); err != nil && errors.Cause(err) != define.ErrDetach {
		return errors.Wrapf(err, "error attaching to container %s", ctr.ID())
	}
	return nil
}

func (ic *ContainerEngine) ContainerExec(ctx context.Context, nameOrId string, options entities.ExecOptions) (int, error) {
	ec := define.ExecErrorCodeGeneric
	if options.PreserveFDs > 0 {
		entries, err := ioutil.ReadDir("/proc/self/fd")
		if err != nil {
			return ec, errors.Wrapf(err, "unable to read /proc/self/fd")
		}

		m := make(map[int]bool)
		for _, e := range entries {
			i, err := strconv.Atoi(e.Name())
			if err != nil {
				return ec, errors.Wrapf(err, "cannot parse %s in /proc/self/fd", e.Name())
			}
			m[i] = true
		}

		for i := 3; i < 3+int(options.PreserveFDs); i++ {
			if _, found := m[i]; !found {
				return ec, errors.New("invalid --preserve-fds=N specified. Not enough FDs available")
			}
		}
	}
	ctrs, err := getContainersByContext(false, options.Latest, []string{nameOrId}, ic.Libpod)
	if err != nil {
		return ec, err
	}
	ctr := ctrs[0]
	ec, err = terminal.ExecAttachCtr(ctx, ctr, options.Tty, options.Privileged, options.Envs, options.Cmd, options.User, options.WorkDir, &options.Streams, options.PreserveFDs, options.DetachKeys)
	return define.TranslateExecErrorToExitCode(ec, err), err
}

func (ic *ContainerEngine) ContainerStart(ctx context.Context, namesOrIds []string, options entities.ContainerStartOptions) ([]*entities.ContainerStartReport, error) {
	var reports []*entities.ContainerStartReport
	var exitCode = define.ExecErrorCodeGeneric
	ctrs, err := getContainersByContext(false, options.Latest, namesOrIds, ic.Libpod)
	if err != nil {
		return nil, err
	}
	// There can only be one container if attach was used
	for _, ctr := range ctrs {
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
					Err:      nil,
					ExitCode: 0,
				})
				return reports, nil
			}

			if errors.Cause(err) == define.ErrWillDeadlock {
				logrus.Debugf("Deadlock error: %v", err)
				reports = append(reports, &entities.ContainerStartReport{
					Id:       ctr.ID(),
					Err:      err,
					ExitCode: define.ExitCode(err),
				})
				return reports, errors.Errorf("attempting to start container %s would cause a deadlock; please run 'podman system renumber' to resolve", ctr.ID())
			}

			if ctrRunning {
				reports = append(reports, &entities.ContainerStartReport{
					Id:       ctr.ID(),
					Err:      nil,
					ExitCode: 0,
				})
				return reports, err
			}

			if err != nil {
				reports = append(reports, &entities.ContainerStartReport{
					Id:       ctr.ID(),
					Err:      err,
					ExitCode: exitCode,
				})
				return reports, errors.Wrapf(err, "unable to start container %s", ctr.ID())
			}

			if ecode, err := ctr.Wait(); err != nil {
				if errors.Cause(err) == define.ErrNoSuchCtr {
					// Check events
					event, err := ic.Libpod.GetLastContainerEvent(ctr.ID(), events.Exited)
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
	return ps.GetContainerLists(ic.Libpod, options)
}

// ContainerDiff provides changes to given container
func (ic *ContainerEngine) ContainerDiff(ctx context.Context, nameOrId string, opts entities.DiffOptions) (*entities.DiffReport, error) {
	if opts.Latest {
		ctnr, err := ic.Libpod.GetLatestContainer()
		if err != nil {
			return nil, errors.Wrap(err, "unable to get latest container")
		}
		nameOrId = ctnr.ID()
	}
	changes, err := ic.Libpod.GetDiff("", nameOrId)
	return &entities.DiffReport{Changes: changes}, err
}

func (ic *ContainerEngine) ContainerRun(ctx context.Context, opts entities.ContainerRunOptions) (*entities.ContainerRunReport, error) {
	var (
		joinPod bool
	)
	if err := generate.CompleteSpec(ctx, ic.Libpod, opts.Spec); err != nil {
		return nil, err
	}
	ctr, err := generate.MakeContainer(ic.Libpod, opts.Spec)
	if err != nil {
		return nil, err
	}

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
			event, err := ic.Libpod.GetLastContainerEvent(ctr.ID(), events.Exited)
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

	if err := ic.Libpod.Log(ctrs, logOpts, logChannel); err != nil {
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
	var reports []*entities.ContainerCleanupReport
	ctrs, err := getContainersByContext(options.All, options.Latest, namesOrIds, ic.Libpod)
	if err != nil {
		return nil, err
	}
	for _, ctr := range ctrs {
		var err error
		report := entities.ContainerCleanupReport{Id: ctr.ID()}
		if options.Remove {
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
