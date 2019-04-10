// +build !remoteclient

package adapter

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/shared"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/adapter/shortcuts"
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
			c.ID(),
			func() error {
				err := c.StopWithTimeout(*timeout)
				if err != nil {
					if errors.Cause(err) == libpod.ErrCtrStopped {
						logrus.Debugf("Container %s is already stopped", c.ID())
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
			c.ID(),
			func() error {
				return c.Kill(uint(signal))
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

	ctrs, err := shortcuts.GetContainersByContext(cli.All, cli.Latest, cli.InputArgs, r.Runtime)
	if err != nil {
		// Force may be used to remove containers no longer found in the database
		if cli.Force && len(cli.InputArgs) > 0 && errors.Cause(err) == libpod.ErrNoSuchCtr {
			r.RemoveContainersFromStorage(cli.InputArgs)
		}
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
		if state == libpod.ContainerStateRunning {
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
func (r *LocalRuntime) Log(c *cliconfig.LogsValues, options *libpod.LogOptions) error {
	var wg sync.WaitGroup
	options.WaitGroup = &wg
	if len(c.InputArgs) > 1 {
		options.Multi = true
	}
	logChannel := make(chan *libpod.LogLine, int(c.Tail)*len(c.InputArgs)+1)
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
	results := shared.NewIntermediateLayer(&c.PodmanCommand)
	ctr, _, err := shared.CreateContainer(ctx, &results, r.Runtime)
	return ctr.ID(), err
}

// Run a libpod container
func (r *LocalRuntime) Run(ctx context.Context, c *cliconfig.RunValues, exitCode int) (int, error) {
	results := shared.NewIntermediateLayer(&c.PodmanCommand)

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
				return exitCode, errors.Wrapf(libpod.ErrInvalidArg, "invalid stream %q for --attach - must be one of stdin, stdout, or stderr", stream)
			}
		}
	}
	// if the container was created as part of a pod, also start its dependencies, if any.
	if err := StartAttachCtr(ctx, ctr, outputStream, errorStream, inputStream, c.String("detach-keys"), c.Bool("sig-proxy"), true, c.IsSet("pod")); err != nil {
		// We've manually detached from the container
		// Do not perform cleanup, or wait for container exit code
		// Just exit immediately
		if errors.Cause(err) == libpod.ErrDetach {
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
		if errors.Cause(err) == libpod.ErrNoSuchCtr {
			// The container may have been removed
			// Go looking for an exit file
			config, err := r.Runtime.GetConfig()
			if err != nil {
				return exitCode, err
			}
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
		r.Runtime.RemoveContainer(ctx, ctr, false, true)
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
