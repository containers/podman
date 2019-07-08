// +build !remoteclient

package adapter

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/containers/buildah"
	"github.com/containers/image/manifest"
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/shared"
	"github.com/containers/libpod/cmd/podman/shared/parse"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/libpod/image"
	"github.com/containers/libpod/libpod/logs"
	"github.com/containers/libpod/pkg/adapter/shortcuts"
	"github.com/containers/libpod/pkg/systemdgen"
	"github.com/containers/psgo"
	"github.com/containers/storage"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// GetLatestContainer gets the latest Container and wraps it in an adapter Container
func (r *LocalRuntime) GetLatestContainer() (*Container, error) {
	Container := Container{}
	c, err := r.Runtime.GetLatestContainer()
	Container.Container = c
	return &Container, err
}

// GetAllContainers gets all Containers and wraps each one in an adapter Container
func (r *LocalRuntime) GetAllContainers() ([]*Container, error) {
	var containers []*Container
	allContainers, err := r.Runtime.GetAllContainers()
	if err != nil {
		return nil, err
	}

	for _, c := range allContainers {
		containers = append(containers, &Container{c})
	}
	return containers, nil
}

// LookupContainer gets a Container by name or id and wraps it in an adapter Container
func (r *LocalRuntime) LookupContainer(idOrName string) (*Container, error) {
	ctr, err := r.Runtime.LookupContainer(idOrName)
	if err != nil {
		return nil, err
	}
	return &Container{ctr}, nil
}

// StopContainers stops container(s) based on CLI inputs.
// Returns list of successful id(s), map of failed id(s) + error, or error not from container
func (r *LocalRuntime) StopContainers(ctx context.Context, cli *cliconfig.StopValues) ([]string, map[string]error, error) {
	var timeout *uint
	if cli.Flags().Changed("timeout") || cli.Flags().Changed("time") {
		t := uint(cli.Timeout)
		timeout = &t
	}

	maxWorkers := shared.DefaultPoolSize("stop")
	if cli.GlobalIsSet("max-workers") {
		maxWorkers = cli.GlobalFlags.MaxWorks
	}
	logrus.Debugf("Setting maximum stop workers to %d", maxWorkers)

	ctrs, err := shortcuts.GetContainersByContext(cli.All, cli.Latest, cli.InputArgs, r.Runtime)
	if err != nil {
		return nil, nil, err
	}

	pool := shared.NewPool("stop", maxWorkers, len(ctrs))
	for _, c := range ctrs {
		c := c

		if timeout == nil {
			t := c.StopTimeout()
			timeout = &t
			logrus.Debugf("Set timeout to container %s default (%d)", c.ID(), *timeout)
		}

		pool.Add(shared.Job{
			ID: c.ID(),
			Fn: func() error {
				err := c.StopWithTimeout(*timeout)
				if err != nil {
					if errors.Cause(err) == define.ErrCtrStopped {
						logrus.Debugf("Container %s is already stopped", c.ID())
						return nil
					} else if cli.All && errors.Cause(err) == define.ErrCtrStateInvalid {
						logrus.Debugf("Container %s is not running, could not stop", c.ID())
						return nil
					}
					logrus.Debugf("Failed to stop container %s: %s", c.ID(), err.Error())
				}
				return err
			},
		})
	}
	return pool.Run()
}

// KillContainers sends signal to container(s) based on CLI inputs.
// Returns list of successful id(s), map of failed id(s) + error, or error not from container
func (r *LocalRuntime) KillContainers(ctx context.Context, cli *cliconfig.KillValues, signal syscall.Signal) ([]string, map[string]error, error) {
	maxWorkers := shared.DefaultPoolSize("kill")
	if cli.GlobalIsSet("max-workers") {
		maxWorkers = cli.GlobalFlags.MaxWorks
	}
	logrus.Debugf("Setting maximum kill workers to %d", maxWorkers)

	ctrs, err := shortcuts.GetContainersByContext(cli.All, cli.Latest, cli.InputArgs, r.Runtime)
	if err != nil {
		return nil, nil, err
	}

	pool := shared.NewPool("kill", maxWorkers, len(ctrs))
	for _, c := range ctrs {
		c := c

		pool.Add(shared.Job{
			ID: c.ID(),
			Fn: func() error {
				return c.Kill(uint(signal))
			},
		})
	}
	return pool.Run()
}

// InitContainers initializes container(s) based on CLI inputs.
// Returns list of successful id(s), map of failed id(s) to errors, or a general
// error not from the container.
func (r *LocalRuntime) InitContainers(ctx context.Context, cli *cliconfig.InitValues) ([]string, map[string]error, error) {
	maxWorkers := shared.DefaultPoolSize("init")
	if cli.GlobalIsSet("max-workers") {
		maxWorkers = cli.GlobalFlags.MaxWorks
	}
	logrus.Debugf("Setting maximum init workers to %d", maxWorkers)

	ctrs, err := shortcuts.GetContainersByContext(cli.All, cli.Latest, cli.InputArgs, r.Runtime)
	if err != nil {
		return nil, nil, err
	}

	pool := shared.NewPool("init", maxWorkers, len(ctrs))
	for _, c := range ctrs {
		ctr := c

		pool.Add(shared.Job{
			ID: ctr.ID(),
			Fn: func() error {
				err := ctr.Init(ctx)
				if err != nil {
					// If we're initializing all containers, ignore invalid state errors
					if cli.All && errors.Cause(err) == define.ErrCtrStateInvalid {
						return nil
					}
					return err
				}
				return nil
			},
		})
	}
	return pool.Run()
}

// RemoveContainers removes container(s) based on CLI inputs.
func (r *LocalRuntime) RemoveContainers(ctx context.Context, cli *cliconfig.RmValues) ([]string, map[string]error, error) {
	var (
		ok       = []string{}
		failures = map[string]error{}
	)

	maxWorkers := shared.DefaultPoolSize("rm")
	if cli.GlobalIsSet("max-workers") {
		maxWorkers = cli.GlobalFlags.MaxWorks
	}
	logrus.Debugf("Setting maximum rm workers to %d", maxWorkers)

	if cli.Storage {
		for _, ctr := range cli.InputArgs {
			if err := r.RemoveStorageContainer(ctr, cli.Force); err != nil {
				failures[ctr] = err
			}
			ok = append(ok, ctr)
		}
		return ok, failures, nil
	}

	ctrs, err := shortcuts.GetContainersByContext(cli.All, cli.Latest, cli.InputArgs, r.Runtime)
	if err != nil {
		return ok, failures, err
	}

	pool := shared.NewPool("rm", maxWorkers, len(ctrs))
	for _, c := range ctrs {
		c := c

		pool.Add(shared.Job{
			c.ID(),
			func() error {
				err := r.RemoveContainer(ctx, c, cli.Force, cli.Volumes)
				if err != nil {
					logrus.Debugf("Failed to remove container %s: %s", c.ID(), err.Error())
				}
				return err
			},
		})
	}
	return pool.Run()
}

// UmountRootFilesystems removes container(s) based on CLI inputs.
func (r *LocalRuntime) UmountRootFilesystems(ctx context.Context, cli *cliconfig.UmountValues) ([]string, map[string]error, error) {
	var (
		ok       = []string{}
		failures = map[string]error{}
	)

	ctrs, err := shortcuts.GetContainersByContext(cli.All, cli.Latest, cli.InputArgs, r.Runtime)
	if err != nil {
		return ok, failures, err
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

		if err := ctr.Unmount(cli.Force); err != nil {
			if cli.All && errors.Cause(err) == storage.ErrLayerNotMounted {
				logrus.Debugf("Error umounting container %s, storage.ErrLayerNotMounted", ctr.ID())
				continue
			}
			failures[ctr.ID()] = errors.Wrapf(err, "error unmounting continaner %s", ctr.ID())
		} else {
			ok = append(ok, ctr.ID())
		}
	}
	return ok, failures, nil
}

// WaitOnContainers waits for all given container(s) to stop
func (r *LocalRuntime) WaitOnContainers(ctx context.Context, cli *cliconfig.WaitValues, interval time.Duration) ([]string, map[string]error, error) {
	var (
		ok       = []string{}
		failures = map[string]error{}
	)

	ctrs, err := shortcuts.GetContainersByContext(false, cli.Latest, cli.InputArgs, r.Runtime)
	if err != nil {
		return ok, failures, err
	}

	for _, c := range ctrs {
		if returnCode, err := c.WaitWithInterval(interval); err == nil {
			ok = append(ok, strconv.Itoa(int(returnCode)))
		} else {
			failures[c.ID()] = err
		}
	}
	return ok, failures, err
}

// Log logs one or more containers
func (r *LocalRuntime) Log(c *cliconfig.LogsValues, options *logs.LogOptions) error {

	var wg sync.WaitGroup
	options.WaitGroup = &wg
	if len(c.InputArgs) > 1 {
		options.Multi = true
	}
	logChannel := make(chan *logs.LogLine, int(c.Tail)*len(c.InputArgs)+1)
	containers, err := shortcuts.GetContainersByContext(false, c.Latest, c.InputArgs, r.Runtime)
	if err != nil {
		return err
	}
	if err := r.Runtime.Log(containers, options, logChannel); err != nil {
		return err
	}
	go func() {
		wg.Wait()
		close(logChannel)
	}()
	for line := range logChannel {
		fmt.Println(line.String(options))
	}
	return nil
}

// CreateContainer creates a libpod container
func (r *LocalRuntime) CreateContainer(ctx context.Context, c *cliconfig.CreateValues) (string, error) {
	results := shared.NewIntermediateLayer(&c.PodmanCommand, false)
	ctr, _, err := shared.CreateContainer(ctx, &results, r.Runtime)
	if err != nil {
		return "", err
	}
	return ctr.ID(), nil
}

// Run a libpod container
func (r *LocalRuntime) Run(ctx context.Context, c *cliconfig.RunValues, exitCode int) (int, error) {
	results := shared.NewIntermediateLayer(&c.PodmanCommand, false)

	ctr, createConfig, err := shared.CreateContainer(ctx, &results, r.Runtime)
	if err != nil {
		return exitCode, err
	}

	if logrus.GetLevel() == logrus.DebugLevel {
		cgroupPath, err := ctr.CGroupPath()
		if err == nil {
			logrus.Debugf("container %q has CgroupParent %q", ctr.ID(), cgroupPath)
		}
	}

	// Handle detached start
	if createConfig.Detach {
		// if the container was created as part of a pod, also start its dependencies, if any.
		if err := ctr.Start(ctx, c.IsSet("pod")); err != nil {
			// This means the command did not exist
			exitCode = 127
			if strings.Index(err.Error(), "permission denied") > -1 {
				exitCode = 126
			}
			return exitCode, err
		}

		fmt.Printf("%s\n", ctr.ID())
		exitCode = 0
		return exitCode, nil
	}

	outputStream := os.Stdout
	errorStream := os.Stderr
	inputStream := os.Stdin

	// If -i is not set, clear stdin
	if !c.Bool("interactive") {
		inputStream = nil
	}

	// If attach is set, clear stdin/stdout/stderr and only attach requested
	if c.IsSet("attach") || c.IsSet("a") {
		outputStream = nil
		errorStream = nil
		if !c.Bool("interactive") {
			inputStream = nil
		}

		attachTo := c.StringSlice("attach")
		for _, stream := range attachTo {
			switch strings.ToLower(stream) {
			case "stdout":
				outputStream = os.Stdout
			case "stderr":
				errorStream = os.Stderr
			case "stdin":
				inputStream = os.Stdin
			default:
				return exitCode, errors.Wrapf(define.ErrInvalidArg, "invalid stream %q for --attach - must be one of stdin, stdout, or stderr", stream)
			}
		}
	}

	config, err := r.Runtime.GetConfig()
	if err != nil {
		return exitCode, err
	}
	detachKeys := c.String("detach-keys")
	if detachKeys == "" {
		detachKeys = config.DetachKeys
	}

	// if the container was created as part of a pod, also start its dependencies, if any.
	if err := StartAttachCtr(ctx, ctr, outputStream, errorStream, inputStream, detachKeys, c.Bool("sig-proxy"), true, c.IsSet("pod")); err != nil {
		// We've manually detached from the container
		// Do not perform cleanup, or wait for container exit code
		// Just exit immediately
		if errors.Cause(err) == define.ErrDetach {
			exitCode = 0
			return exitCode, nil
		}
		// This means the command did not exist
		exitCode = 127
		if strings.Index(err.Error(), "permission denied") > -1 {
			exitCode = 126
		}
		if c.IsSet("rm") {
			if deleteError := r.Runtime.RemoveContainer(ctx, ctr, true, false); deleteError != nil {
				logrus.Errorf("unable to remove container %s after failing to start and attach to it", ctr.ID())
			}
		}
		return exitCode, err
	}

	if ecode, err := ctr.Wait(); err != nil {
		if errors.Cause(err) == define.ErrNoSuchCtr {
			// The container may have been removed
			// Go looking for an exit file
			ctrExitCode, err := ReadExitFile(config.TmpDir, ctr.ID())
			if err != nil {
				logrus.Errorf("Cannot get exit code: %v", err)
				exitCode = 127
			} else {
				exitCode = ctrExitCode
			}
		}
	} else {
		exitCode = int(ecode)
	}

	if c.IsSet("rm") {
		if err := r.Runtime.RemoveContainer(ctx, ctr, false, false); err != nil {
			logrus.Errorf("Error removing container %s: %v", ctr.ID(), err)
		}
	}

	return exitCode, nil
}

// ReadExitFile reads a container's exit file
func ReadExitFile(runtimeTmp, ctrID string) (int, error) {
	exitFile := filepath.Join(runtimeTmp, "exits", fmt.Sprintf("%s-old", ctrID))

	logrus.Debugf("Attempting to read container %s exit code from file %s", ctrID, exitFile)

	// Check if it exists
	if _, err := os.Stat(exitFile); err != nil {
		return 0, errors.Wrapf(err, "error getting exit file for container %s", ctrID)
	}

	// File exists, read it in and convert to int
	statusStr, err := ioutil.ReadFile(exitFile)
	if err != nil {
		return 0, errors.Wrapf(err, "error reading exit file for container %s", ctrID)
	}

	exitCode, err := strconv.Atoi(string(statusStr))
	if err != nil {
		return 0, errors.Wrapf(err, "error parsing exit code for container %s", ctrID)
	}

	return exitCode, nil
}

// Ps ...
func (r *LocalRuntime) Ps(c *cliconfig.PsValues, opts shared.PsOptions) ([]shared.PsContainerOutput, error) {
	maxWorkers := shared.Parallelize("ps")
	if c.GlobalIsSet("max-workers") {
		maxWorkers = c.GlobalFlags.MaxWorks
	}
	logrus.Debugf("Setting maximum workers to %d", maxWorkers)
	return shared.GetPsContainerOutput(r.Runtime, opts, c.Filter, maxWorkers)
}

// Attach ...
func (r *LocalRuntime) Attach(ctx context.Context, c *cliconfig.AttachValues) error {
	var (
		ctr *libpod.Container
		err error
	)

	if c.Latest {
		ctr, err = r.Runtime.GetLatestContainer()
	} else {
		ctr, err = r.Runtime.LookupContainer(c.InputArgs[0])
	}

	if err != nil {
		return errors.Wrapf(err, "unable to exec into %s", c.InputArgs[0])
	}

	conState, err := ctr.State()
	if err != nil {
		return errors.Wrapf(err, "unable to determine state of %s", ctr.ID())
	}
	if conState != define.ContainerStateRunning {
		return errors.Errorf("you can only attach to running containers")
	}

	inputStream := os.Stdin
	if c.NoStdin {
		inputStream = nil
	}
	// If the container is in a pod, also set to recursively start dependencies
	if err := StartAttachCtr(ctx, ctr, os.Stdout, os.Stderr, inputStream, c.DetachKeys, c.SigProxy, false, ctr.PodID() != ""); err != nil && errors.Cause(err) != define.ErrDetach {
		return errors.Wrapf(err, "error attaching to container %s", ctr.ID())
	}
	return nil
}

// Checkpoint one or more containers
func (r *LocalRuntime) Checkpoint(c *cliconfig.CheckpointValues) error {
	var (
		containers     []*libpod.Container
		err, lastError error
	)

	options := libpod.ContainerCheckpointOptions{
		Keep:           c.Keep,
		KeepRunning:    c.LeaveRunning,
		TCPEstablished: c.TcpEstablished,
		TargetFile:     c.Export,
	}
	if c.All {
		containers, err = r.Runtime.GetRunningContainers()
	} else {
		containers, err = shortcuts.GetContainersByContext(false, c.Latest, c.InputArgs, r.Runtime)
	}
	if err != nil {
		return err
	}

	for _, ctr := range containers {
		if err = ctr.Checkpoint(context.TODO(), options); err != nil {
			if lastError != nil {
				fmt.Fprintln(os.Stderr, lastError)
			}
			lastError = errors.Wrapf(err, "failed to checkpoint container %v", ctr.ID())
		} else {
			fmt.Println(ctr.ID())
		}
	}
	return lastError
}

// Restore one or more containers
func (r *LocalRuntime) Restore(ctx context.Context, c *cliconfig.RestoreValues) error {
	var (
		containers     []*libpod.Container
		err, lastError error
		filterFuncs    []libpod.ContainerFilter
	)

	options := libpod.ContainerCheckpointOptions{
		Keep:           c.Keep,
		TCPEstablished: c.TcpEstablished,
		TargetFile:     c.Import,
		Name:           c.Name,
	}

	filterFuncs = append(filterFuncs, func(c *libpod.Container) bool {
		state, _ := c.State()
		return state == define.ContainerStateExited
	})

	if c.Import != "" {
		containers, err = crImportCheckpoint(ctx, r.Runtime, c.Import, c.Name)
	} else if c.All {
		containers, err = r.GetContainers(filterFuncs...)
	} else {
		containers, err = shortcuts.GetContainersByContext(false, c.Latest, c.InputArgs, r.Runtime)
	}
	if err != nil {
		return err
	}

	for _, ctr := range containers {
		if err = ctr.Restore(context.TODO(), options); err != nil {
			if lastError != nil {
				fmt.Fprintln(os.Stderr, lastError)
			}
			lastError = errors.Wrapf(err, "failed to restore container %v", ctr.ID())
		} else {
			fmt.Println(ctr.ID())
		}
	}
	return lastError
}

// Start will start a container
func (r *LocalRuntime) Start(ctx context.Context, c *cliconfig.StartValues, sigProxy bool) (int, error) {
	var (
		exitCode  = 125
		lastError error
	)

	args := c.InputArgs
	if c.Latest {
		lastCtr, err := r.GetLatestContainer()
		if err != nil {
			return 0, errors.Wrapf(err, "unable to get latest container")
		}
		args = append(args, lastCtr.ID())
	}

	for _, container := range args {
		ctr, err := r.LookupContainer(container)
		if err != nil {
			if lastError != nil {
				fmt.Fprintln(os.Stderr, lastError)
			}
			lastError = errors.Wrapf(err, "unable to find container %s", container)
			continue
		}

		ctrState, err := ctr.State()
		if err != nil {
			return exitCode, errors.Wrapf(err, "unable to get container state")
		}

		ctrRunning := ctrState == define.ContainerStateRunning

		if c.Attach {
			inputStream := os.Stdin
			if !c.Interactive {
				inputStream = nil
			}

			// attach to the container and also start it not already running
			// If the container is in a pod, also set to recursively start dependencies
			err = StartAttachCtr(ctx, ctr.Container, os.Stdout, os.Stderr, inputStream, c.DetachKeys, sigProxy, !ctrRunning, ctr.PodID() != "")
			if errors.Cause(err) == define.ErrDetach {
				// User manually detached
				// Exit cleanly immediately
				exitCode = 0
				return exitCode, nil
			}

			if ctrRunning {
				return 0, err
			}

			if err != nil {
				return exitCode, errors.Wrapf(err, "unable to start container %s", ctr.ID())
			}

			if ecode, err := ctr.Wait(); err != nil {
				if errors.Cause(err) == define.ErrNoSuchCtr {
					// The container may have been removed
					// Go looking for an exit file
					rtc, err := r.GetConfig()
					if err != nil {
						return 0, err
					}
					ctrExitCode, err := ReadExitFile(rtc.TmpDir, ctr.ID())
					if err != nil {
						logrus.Errorf("Cannot get exit code: %v", err)
						exitCode = 127
					} else {
						exitCode = ctrExitCode
					}
				}
			} else {
				exitCode = int(ecode)
			}

			return exitCode, nil
		}
		if ctrRunning {
			fmt.Println(ctr.ID())
			continue
		}
		// Handle non-attach start
		// If the container is in a pod, also set to recursively start dependencies
		if err := ctr.Start(ctx, ctr.PodID() != ""); err != nil {
			if lastError != nil {
				fmt.Fprintln(os.Stderr, lastError)
			}
			lastError = errors.Wrapf(err, "unable to start container %q", container)
			continue
		}
		fmt.Println(container)
	}
	return exitCode, lastError
}

// PauseContainers removes container(s) based on CLI inputs.
func (r *LocalRuntime) PauseContainers(ctx context.Context, cli *cliconfig.PauseValues) ([]string, map[string]error, error) {
	var (
		ok       = []string{}
		failures = map[string]error{}
		ctrs     []*libpod.Container
		err      error
	)

	maxWorkers := shared.DefaultPoolSize("pause")
	if cli.GlobalIsSet("max-workers") {
		maxWorkers = cli.GlobalFlags.MaxWorks
	}
	logrus.Debugf("Setting maximum rm workers to %d", maxWorkers)

	if cli.All {
		ctrs, err = r.GetRunningContainers()
	} else {
		ctrs, err = shortcuts.GetContainersByContext(false, false, cli.InputArgs, r.Runtime)
	}
	if err != nil {
		return ok, failures, err
	}

	pool := shared.NewPool("pause", maxWorkers, len(ctrs))
	for _, c := range ctrs {
		ctr := c
		pool.Add(shared.Job{
			ID: ctr.ID(),
			Fn: func() error {
				err := ctr.Pause()
				if err != nil {
					logrus.Debugf("Failed to pause container %s: %s", ctr.ID(), err.Error())
				}
				return err
			},
		})
	}
	return pool.Run()
}

// UnpauseContainers removes container(s) based on CLI inputs.
func (r *LocalRuntime) UnpauseContainers(ctx context.Context, cli *cliconfig.UnpauseValues) ([]string, map[string]error, error) {
	var (
		ok       = []string{}
		failures = map[string]error{}
		ctrs     []*libpod.Container
		err      error
	)

	maxWorkers := shared.DefaultPoolSize("pause")
	if cli.GlobalIsSet("max-workers") {
		maxWorkers = cli.GlobalFlags.MaxWorks
	}
	logrus.Debugf("Setting maximum rm workers to %d", maxWorkers)

	if cli.All {
		var filterFuncs []libpod.ContainerFilter
		filterFuncs = append(filterFuncs, func(c *libpod.Container) bool {
			state, _ := c.State()
			return state == define.ContainerStatePaused
		})
		ctrs, err = r.GetContainers(filterFuncs...)
	} else {
		ctrs, err = shortcuts.GetContainersByContext(false, false, cli.InputArgs, r.Runtime)
	}
	if err != nil {
		return ok, failures, err
	}

	pool := shared.NewPool("pause", maxWorkers, len(ctrs))
	for _, c := range ctrs {
		ctr := c
		pool.Add(shared.Job{
			ID: ctr.ID(),
			Fn: func() error {
				err := ctr.Unpause()
				if err != nil {
					logrus.Debugf("Failed to unpause container %s: %s", ctr.ID(), err.Error())
				}
				return err
			},
		})
	}
	return pool.Run()
}

// Restart containers without or without a timeout
func (r *LocalRuntime) Restart(ctx context.Context, c *cliconfig.RestartValues) ([]string, map[string]error, error) {
	var (
		containers        []*libpod.Container
		restartContainers []*libpod.Container
		err               error
	)
	useTimeout := c.Flag("timeout").Changed || c.Flag("time").Changed
	inputTimeout := c.Timeout

	// Handle --latest
	if c.Latest {
		lastCtr, err := r.Runtime.GetLatestContainer()
		if err != nil {
			return nil, nil, errors.Wrapf(err, "unable to get latest container")
		}
		restartContainers = append(restartContainers, lastCtr)
	} else if c.Running {
		containers, err = r.GetRunningContainers()
		if err != nil {
			return nil, nil, err
		}
		restartContainers = append(restartContainers, containers...)
	} else if c.All {
		containers, err = r.Runtime.GetAllContainers()
		if err != nil {
			return nil, nil, err
		}
		restartContainers = append(restartContainers, containers...)
	} else {
		for _, id := range c.InputArgs {
			ctr, err := r.Runtime.LookupContainer(id)
			if err != nil {
				return nil, nil, err
			}
			restartContainers = append(restartContainers, ctr)
		}
	}

	maxWorkers := shared.DefaultPoolSize("restart")
	if c.GlobalIsSet("max-workers") {
		maxWorkers = c.GlobalFlags.MaxWorks
	}

	logrus.Debugf("Setting maximum workers to %d", maxWorkers)

	// We now have a slice of all the containers to be restarted. Iterate them to
	// create restart Funcs with a timeout as needed
	pool := shared.NewPool("restart", maxWorkers, len(restartContainers))
	for _, c := range restartContainers {
		ctr := c
		timeout := ctr.StopTimeout()
		if useTimeout {
			timeout = inputTimeout
		}
		pool.Add(shared.Job{
			ID: ctr.ID(),
			Fn: func() error {
				err := ctr.RestartWithTimeout(ctx, timeout)
				if err != nil {
					logrus.Debugf("Failed to restart container %s: %s", ctr.ID(), err.Error())
				}
				return err
			},
		})
	}
	return pool.Run()
}

// Top display the running processes of a container
func (r *LocalRuntime) Top(cli *cliconfig.TopValues) ([]string, error) {
	var (
		descriptors []string
		container   *libpod.Container
		err         error
	)
	if cli.Latest {
		descriptors = cli.InputArgs
		container, err = r.Runtime.GetLatestContainer()
	} else {
		descriptors = cli.InputArgs[1:]
		container, err = r.Runtime.LookupContainer(cli.InputArgs[0])
	}
	if err != nil {
		return nil, errors.Wrapf(err, "unable to lookup requested container")
	}

	output, psgoErr := container.Top(descriptors)
	if psgoErr == nil {
		return output, nil
	}

	// If we encountered an ErrUnknownDescriptor error, fallback to executing
	// ps(1). This ensures backwards compatibility to users depending on ps(1)
	// and makes sure we're ~compatible with docker.
	if errors.Cause(psgoErr) != psgo.ErrUnknownDescriptor {
		return nil, psgoErr
	}

	output, err = r.execPS(container, descriptors)
	if err != nil {
		// Note: return psgoErr to guide users into using the AIX descriptors
		// instead of using ps(1).
		return nil, psgoErr
	}

	// Trick: filter the ps command from the output instead of
	// checking/requiring PIDs in the output.
	filtered := []string{}
	cmd := strings.Join(descriptors, " ")
	for _, line := range output {
		if !strings.Contains(line, cmd) {
			filtered = append(filtered, line)
		}
	}

	return filtered, nil
}

func (r *LocalRuntime) execPS(c *libpod.Container, args []string) ([]string, error) {
	rPipe, wPipe, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	defer wPipe.Close()
	defer rPipe.Close()

	streams := new(libpod.AttachStreams)
	streams.OutputStream = wPipe
	streams.ErrorStream = wPipe
	streams.InputStream = os.Stdin
	streams.AttachOutput = true
	streams.AttachError = true
	streams.AttachInput = true

	psOutput := []string{}
	go func() {
		scanner := bufio.NewScanner(rPipe)
		for scanner.Scan() {
			psOutput = append(psOutput, scanner.Text())
		}
	}()

	cmd := append([]string{"ps"}, args...)
	if err := c.Exec(false, false, []string{}, cmd, "", "", streams, 0); err != nil {
		return nil, err
	}

	return psOutput, nil
}

// Prune removes stopped containers
func (r *LocalRuntime) Prune(ctx context.Context, maxWorkers int, force bool) ([]string, map[string]error, error) {
	var (
		ok       = []string{}
		failures = map[string]error{}
		err      error
	)

	logrus.Debugf("Setting maximum rm workers to %d", maxWorkers)

	filter := func(c *libpod.Container) bool {
		state, err := c.State()
		if err != nil {
			logrus.Error(err)
			return false
		}
		if c.PodID() != "" {
			return false
		}
		if state == define.ContainerStateStopped || state == define.ContainerStateExited {
			return true
		}
		return false
	}
	delContainers, err := r.Runtime.GetContainers(filter)
	if err != nil {
		return ok, failures, err
	}
	if len(delContainers) < 1 {
		return ok, failures, err
	}
	pool := shared.NewPool("prune", maxWorkers, len(delContainers))
	for _, c := range delContainers {
		ctr := c
		pool.Add(shared.Job{
			ID: ctr.ID(),
			Fn: func() error {
				err := r.Runtime.RemoveContainer(ctx, ctr, force, false)
				if err != nil {
					logrus.Debugf("Failed to prune container %s: %s", ctr.ID(), err.Error())
				}
				return err
			},
		})
	}
	return pool.Run()
}

// CleanupContainers any leftovers bits of stopped containers
func (r *LocalRuntime) CleanupContainers(ctx context.Context, cli *cliconfig.CleanupValues) ([]string, map[string]error, error) {
	var (
		ok       = []string{}
		failures = map[string]error{}
	)

	ctrs, err := shortcuts.GetContainersByContext(cli.All, cli.Latest, cli.InputArgs, r.Runtime)
	if err != nil {
		return ok, failures, err
	}

	for _, ctr := range ctrs {
		if cli.Remove {
			err = removeContainer(ctx, ctr, r)
		} else {
			err = cleanupContainer(ctx, ctr, r)
		}

		if err == nil {
			ok = append(ok, ctr.ID())
		} else {
			failures[ctr.ID()] = err
		}
	}
	return ok, failures, nil
}

// Only used when cleaning up containers
func removeContainer(ctx context.Context, ctr *libpod.Container, runtime *LocalRuntime) error {
	if err := runtime.RemoveContainer(ctx, ctr, false, false); err != nil {
		return errors.Wrapf(err, "failed to cleanup and remove container %v", ctr.ID())
	}
	return nil
}

func cleanupContainer(ctx context.Context, ctr *libpod.Container, runtime *LocalRuntime) error {
	if err := ctr.Cleanup(ctx); err != nil {
		return errors.Wrapf(err, "failed to cleanup container %v", ctr.ID())
	}
	return nil
}

// Port displays port information about existing containers
func (r *LocalRuntime) Port(c *cliconfig.PortValues) ([]*Container, error) {
	var (
		portContainers []*Container
		containers     []*libpod.Container
		err            error
	)

	if !c.All {
		containers, err = shortcuts.GetContainersByContext(false, c.Latest, c.InputArgs, r.Runtime)
	} else {
		containers, err = r.Runtime.GetRunningContainers()
	}
	if err != nil {
		return nil, err
	}

	//Convert libpod containers to adapter Containers
	for _, con := range containers {
		if state, _ := con.State(); state != define.ContainerStateRunning {
			continue
		}
		portContainers = append(portContainers, &Container{con})
	}
	return portContainers, nil
}

// GenerateSystemd creates a unit file for a container
func (r *LocalRuntime) GenerateSystemd(c *cliconfig.GenerateSystemdValues) (string, error) {
	ctr, err := r.Runtime.LookupContainer(c.InputArgs[0])
	if err != nil {
		return "", err
	}
	timeout := int(ctr.StopTimeout())
	if c.StopTimeout >= 0 {
		timeout = int(c.StopTimeout)
	}
	name := ctr.ID()
	if c.Name {
		name = ctr.Name()
	}

	config := ctr.Config()
	conmonPidFile := config.ConmonPidFile
	if conmonPidFile == "" {
		return "", errors.Errorf("conmon PID file path is empty, try to recreate the container with --conmon-pidfile flag")
	}

	return systemdgen.CreateSystemdUnitAsString(name, ctr.ID(), c.RestartPolicy, conmonPidFile, timeout)
}

// GetNamespaces returns namespace information about a container for PS
func (r *LocalRuntime) GetNamespaces(container shared.PsContainerOutput) *shared.Namespace {
	return shared.GetNamespaces(container.Pid)
}

// Commit creates a local image from a container
func (r *LocalRuntime) Commit(ctx context.Context, c *cliconfig.CommitValues, container, imageName string) (string, error) {
	var (
		writer   io.Writer
		mimeType string
	)
	switch c.Format {
	case "oci":
		mimeType = buildah.OCIv1ImageManifest
		if c.Flag("message").Changed {
			return "", errors.Errorf("messages are only compatible with the docker image format (-f docker)")
		}
	case "docker":
		mimeType = manifest.DockerV2Schema2MediaType
	default:
		return "", errors.Errorf("unrecognized image format %q", c.Format)
	}
	if !c.Quiet {
		writer = os.Stderr
	}
	ctr, err := r.Runtime.LookupContainer(container)
	if err != nil {
		return "", errors.Wrapf(err, "error looking up container %q", container)
	}

	rtc, err := r.Runtime.GetConfig()
	if err != nil {
		return "", err
	}

	sc := image.GetSystemContext(rtc.SignaturePolicyPath, "", false)
	coptions := buildah.CommitOptions{
		SignaturePolicyPath:   rtc.SignaturePolicyPath,
		ReportWriter:          writer,
		SystemContext:         sc,
		PreferredManifestType: mimeType,
	}
	options := libpod.ContainerCommitOptions{
		CommitOptions:  coptions,
		Pause:          c.Pause,
		IncludeVolumes: c.IncludeVolumes,
		Message:        c.Message,
		Changes:        c.Change,
		Author:         c.Author,
	}
	newImage, err := ctr.Commit(ctx, imageName, options)
	if err != nil {
		return "", err
	}
	return newImage.ID(), nil
}

// Exec a command in a container
func (r *LocalRuntime) Exec(c *cliconfig.ExecValues, cmd []string) error {
	var ctr *Container
	var err error

	if c.Latest {
		ctr, err = r.GetLatestContainer()
	} else {
		ctr, err = r.LookupContainer(c.InputArgs[0])
	}
	if err != nil {
		return errors.Wrapf(err, "unable to exec into %s", c.InputArgs[0])
	}

	if c.PreserveFDs > 0 {
		entries, err := ioutil.ReadDir("/proc/self/fd")
		if err != nil {
			return errors.Wrapf(err, "unable to read /proc/self/fd")
		}
		m := make(map[int]bool)
		for _, e := range entries {
			i, err := strconv.Atoi(e.Name())
			if err != nil {
				if err != nil {
					return errors.Wrapf(err, "cannot parse %s in /proc/self/fd", e.Name())
				}
			}
			m[i] = true
		}
		for i := 3; i < 3+c.PreserveFDs; i++ {
			if _, found := m[i]; !found {
				return errors.New("invalid --preserve-fds=N specified. Not enough FDs available")
			}
		}
	}

	// ENVIRONMENT VARIABLES
	env := map[string]string{}

	if err := parse.ReadKVStrings(env, []string{}, c.Env); err != nil {
		return errors.Wrapf(err, "unable to process environment variables")
	}
	envs := []string{}
	for k, v := range env {
		envs = append(envs, fmt.Sprintf("%s=%s", k, v))
	}

	streams := new(libpod.AttachStreams)
	streams.OutputStream = os.Stdout
	streams.ErrorStream = os.Stderr
	streams.InputStream = os.Stdin
	streams.AttachOutput = true
	streams.AttachError = true
	streams.AttachInput = true

	return ctr.Exec(c.Tty, c.Privileged, envs, cmd, c.User, c.Workdir, streams, c.PreserveFDs)
}
