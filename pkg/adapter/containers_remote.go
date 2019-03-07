// +build remoteclient

package adapter

import (
	"context"
	"encoding/json"
	"strconv"
	"syscall"
	"time"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/shared"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	iopodman "github.com/containers/libpod/cmd/podman/varlink"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/inspect"
)

// Inspect returns an inspect struct from varlink
func (c *Container) Inspect(size bool) (*inspect.ContainerInspectData, error) {
	reply, err := iopodman.ContainerInspectData().Call(c.Runtime.Conn, c.ID())
	if err != nil {
		return nil, err
	}
	data := inspect.ContainerInspectData{}
	if err := json.Unmarshal([]byte(reply), &data); err != nil {
		return nil, err
	}
	return &data, err
}

// ID returns the ID of the container
func (c *Container) ID() string {
	return c.config.ID
}

// Config returns a container config
func (r *LocalRuntime) Config(name string) *libpod.ContainerConfig {
	// TODO the Spec being returned is not populated.  Matt and I could not figure out why.  Will defer
	// further looking into it for after devconf.
	// The libpod function for this has no errors so we are kind of in a tough
	// spot here.  Logging the errors for now.
	reply, err := iopodman.ContainerConfig().Call(r.Conn, name)
	if err != nil {
		logrus.Error("call to container.config failed")
	}
	data := libpod.ContainerConfig{}
	if err := json.Unmarshal([]byte(reply), &data); err != nil {
		logrus.Error("failed to unmarshal container inspect data")
	}
	return &data

}

// ContainerState returns the "state" of the container.
func (r *LocalRuntime) ContainerState(name string) (*libpod.ContainerState, error) { // no-lint
	reply, err := iopodman.ContainerStateData().Call(r.Conn, name)
	if err != nil {
		return nil, err
	}
	data := libpod.ContainerState{}
	if err := json.Unmarshal([]byte(reply), &data); err != nil {
		return nil, err
	}
	return &data, err

}

// LookupContainer gets basic information about container over a varlink
// connection and then translates it to a *Container
func (r *LocalRuntime) LookupContainer(idOrName string) (*Container, error) {
	state, err := r.ContainerState(idOrName)
	if err != nil {
		return nil, err
	}
	config := r.Config(idOrName)
	if err != nil {
		return nil, err
	}

	return &Container{
		remoteContainer{
			r,
			config,
			state,
		},
	}, nil
}

func (r *LocalRuntime) GetLatestContainer() (*Container, error) {
	reply, err := iopodman.GetContainersByContext().Call(r.Conn, false, true, nil)
	if err != nil {
		return nil, err
	}
	if len(reply) > 0 {
		return r.LookupContainer(reply[0])
	}
	return nil, errors.New("no containers exist")
}

// GetArtifact returns a container's artifacts
func (c *Container) GetArtifact(name string) ([]byte, error) {
	var data []byte
	reply, err := iopodman.ContainerArtifacts().Call(c.Runtime.Conn, c.ID(), name)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(reply), &data); err != nil {
		return nil, err
	}
	return data, err
}

// Config returns a container's Config ... same as ctr.Config()
func (c *Container) Config() *libpod.ContainerConfig {
	if c.config != nil {
		return c.config
	}
	return c.Runtime.Config(c.ID())
}

// Name returns the name of the container
func (c *Container) Name() string {
	return c.config.Name
}

// StopContainers stops requested containers using varlink.
// Returns the list of stopped container ids, map of failed to stop container ids + errors, or any non-container error
func (r *LocalRuntime) StopContainers(ctx context.Context, cli *cliconfig.StopValues) ([]string, map[string]error, error) {
	var (
		ok       = []string{}
		failures = map[string]error{}
	)

	ids, err := iopodman.GetContainersByContext().Call(r.Conn, cli.All, cli.Latest, cli.InputArgs)
	if err != nil {
		return ok, failures, err
	}

	for _, id := range ids {
		stopped, err := iopodman.StopContainer().Call(r.Conn, id, int64(cli.Timeout))
		if err != nil {
			failures[id] = err
		} else {
			ok = append(ok, stopped)
		}
	}
	return ok, failures, nil
}

// KillContainers sends signal to container(s) based on CLI inputs.
// Returns list of successful id(s), map of failed id(s) + error, or error not from container
func (r *LocalRuntime) KillContainers(ctx context.Context, cli *cliconfig.KillValues, signal syscall.Signal) ([]string, map[string]error, error) {
	var (
		ok       = []string{}
		failures = map[string]error{}
	)

	ids, err := iopodman.GetContainersByContext().Call(r.Conn, cli.All, cli.Latest, cli.InputArgs)
	if err != nil {
		return ok, failures, err
	}

	for _, id := range ids {
		killed, err := iopodman.KillContainer().Call(r.Conn, id, int64(signal))
		if err != nil {
			failures[id] = err
		} else {
			ok = append(ok, killed)
		}
	}
	return ok, failures, nil
}

// RemoveContainer removes container(s) based on varlink inputs.
func (r *LocalRuntime) RemoveContainers(ctx context.Context, cli *cliconfig.RmValues) ([]string, map[string]error, error) {
	ids, err := iopodman.GetContainersByContext().Call(r.Conn, cli.All, cli.Latest, cli.InputArgs)
	if err != nil {
		return nil, nil, err
	}

	var (
		ok       = []string{}
		failures = map[string]error{}
	)

	for _, id := range ids {
		_, err := iopodman.RemoveContainer().Call(r.Conn, id, cli.Force, cli.Volumes)
		if err != nil {
			failures[id] = err
		} else {
			ok = append(ok, id)
		}
	}
	return ok, failures, nil
}

// WaitOnContainers waits for all given container(s) to stop.
// interval is currently ignored.
func (r *LocalRuntime) WaitOnContainers(ctx context.Context, cli *cliconfig.WaitValues, interval time.Duration) ([]string, map[string]error, error) {
	var (
		ok       = []string{}
		failures = map[string]error{}
	)

	ids, err := iopodman.GetContainersByContext().Call(r.Conn, false, cli.Latest, cli.InputArgs)
	if err != nil {
		return ok, failures, err
	}

	for _, id := range ids {
		stopped, err := iopodman.WaitContainer().Call(r.Conn, id, int64(interval))
		if err != nil {
			failures[id] = err
		} else {
			ok = append(ok, strconv.FormatInt(stopped, 10))
		}
	}
	return ok, failures, nil
}

// BatchContainerOp is wrapper func to mimic shared's function with a similar name meant for libpod
func BatchContainerOp(ctr *Container, opts shared.PsOptions) (shared.BatchContainerStruct, error) {
	// TODO If pod ps ever shows container's sizes, re-enable this code; otherwise it isn't needed
	// and would be a perf hit
	// data, err := ctr.Inspect(true)
	// if err != nil {
	// 	return shared.BatchContainerStruct{}, err
	// }
	//
	// size := new(shared.ContainerSize)
	// size.RootFsSize = data.SizeRootFs
	// size.RwSize = data.SizeRw

	bcs := shared.BatchContainerStruct{
		ConConfig:   ctr.config,
		ConState:    ctr.state.State,
		ExitCode:    ctr.state.ExitCode,
		Pid:         ctr.state.PID,
		StartedTime: ctr.state.StartedTime,
		ExitedTime:  ctr.state.FinishedTime,
		// Size: size,
	}
	return bcs, nil
}
