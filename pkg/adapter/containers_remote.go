// +build remoteclient

package adapter

import (
	"context"
	"encoding/json"
	"fmt"
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
	"github.com/varlink/go/varlink"
)

// Inspect returns an inspect struct from varlink
func (c *Container) Inspect(size bool) (*inspect.ContainerInspectData, error) {
	reply, err := iopodman.ContainerInspectData().Call(c.Runtime.Conn, c.ID(), size)
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

// KillContainers sends signal to container(s) based on varlink.
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

// UmountRootFilesystems umounts container(s) root filesystems based on varlink inputs
func (r *LocalRuntime) UmountRootFilesystems(ctx context.Context, cli *cliconfig.UmountValues) ([]string, map[string]error, error) {
	ids, err := iopodman.GetContainersByContext().Call(r.Conn, cli.All, cli.Latest, cli.InputArgs)
	if err != nil {
		return nil, nil, err
	}

	var (
		ok       = []string{}
		failures = map[string]error{}
	)

	for _, id := range ids {
		err := iopodman.UnmountContainer().Call(r.Conn, id, cli.Force)
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

// Logs one or more containers over a varlink connection
func (r *LocalRuntime) Log(c *cliconfig.LogsValues, options *libpod.LogOptions) error {
	// GetContainersLogs
	reply, err := iopodman.GetContainersLogs().Send(r.Conn, uint64(varlink.More), c.InputArgs, c.Follow, c.Latest, options.Since.Format(time.RFC3339Nano), int64(c.Tail), c.Timestamps)
	if err != nil {
		return errors.Wrapf(err, "failed to get container logs")
	}
	if len(c.InputArgs) > 1 {
		options.Multi = true
	}
	for {
		log, flags, err := reply()
		if err != nil {
			return err
		}
		if log.Time == "" && log.Msg == "" {
			// We got a blank log line which can signal end of stream
			break
		}
		lTime, err := time.Parse(time.RFC3339Nano, log.Time)
		if err != nil {
			return errors.Wrapf(err, "unable to parse time of log %s", log.Time)
		}
		logLine := libpod.LogLine{
			Device:       log.Device,
			ParseLogType: log.ParseLogType,
			Time:         lTime,
			Msg:          log.Msg,
			CID:          log.Cid,
		}
		fmt.Println(logLine.String(options))
		if flags&varlink.Continues == 0 {
			break
		}
	}
	return nil
}

// CreateContainer creates a container from the cli over varlink
func (r *LocalRuntime) CreateContainer(ctx context.Context, c *cliconfig.CreateValues) (string, error) {
	if !c.Bool("detach") {
		// TODO need to add attach when that function becomes available
		return "", errors.New("the remote client only supports detached containers")
	}
	results := shared.NewIntermediateLayer(&c.PodmanCommand)
	return iopodman.CreateContainer().Call(r.Conn, results.MakeVarlink())
}

// Run creates a container overvarlink and then starts it
func (r *LocalRuntime) Run(ctx context.Context, c *cliconfig.RunValues, exitCode int) (int, error) {
	// TODO the exit codes for run need to be figured out for remote connections
	if !c.Bool("detach") {
		return 0, errors.New("the remote client only supports detached containers")
	}
	results := shared.NewIntermediateLayer(&c.PodmanCommand)
	cid, err := iopodman.CreateContainer().Call(r.Conn, results.MakeVarlink())
	if err != nil {
		return 0, err
	}
	fmt.Println(cid)
	_, err = iopodman.StartContainer().Call(r.Conn, cid)
	return 0, err
}

func ReadExitFile(runtimeTmp, ctrID string) (int, error) {
	return 0, libpod.ErrNotImplemented
}

// Ps ...
func (r *LocalRuntime) Ps(c *cliconfig.PsValues, opts shared.PsOptions) ([]shared.PsContainerOutput, error) {
	var psContainers []shared.PsContainerOutput
	last := int64(c.Last)
	PsOpts := iopodman.PsOpts{
		All:     c.All,
		Filters: &c.Filter,
		Last:    &last,
		Latest:  &c.Latest,
		NoTrunc: &c.NoTrunct,
		Pod:     &c.Pod,
		Quiet:   &c.Quiet,
		Sort:    &c.Sort,
		Sync:    &c.Sync,
	}
	containers, err := iopodman.Ps().Call(r.Conn, PsOpts)
	if err != nil {
		return nil, err
	}
	for _, ctr := range containers {
		createdAt, err := time.Parse(time.RFC3339Nano, ctr.CreatedAt)
		if err != nil {
			return nil, err
		}
		exitedAt, err := time.Parse(time.RFC3339Nano, ctr.ExitedAt)
		if err != nil {
			return nil, err
		}
		startedAt, err := time.Parse(time.RFC3339Nano, ctr.StartedAt)
		if err != nil {
			return nil, err
		}
		containerSize := shared.ContainerSize{
			RootFsSize: ctr.RootFsSize,
			RwSize:     ctr.RwSize,
		}
		state, err := libpod.StringToContainerStatus(ctr.State)
		if err != nil {
			return nil, err
		}
		psc := shared.PsContainerOutput{
			ID:        ctr.Id,
			Image:     ctr.Image,
			Command:   ctr.Command,
			Created:   ctr.Created,
			Ports:     ctr.Ports,
			Names:     ctr.Names,
			IsInfra:   ctr.IsInfra,
			Status:    ctr.Status,
			State:     state,
			Pid:       int(ctr.PidNum),
			Size:      &containerSize,
			Pod:       ctr.Pod,
			CreatedAt: createdAt,
			ExitedAt:  exitedAt,
			StartedAt: startedAt,
			Labels:    ctr.Labels,
			PID:       ctr.NsPid,
			Cgroup:    ctr.Cgroup,
			IPC:       ctr.Ipc,
			MNT:       ctr.Mnt,
			NET:       ctr.Net,
			PIDNS:     ctr.PidNs,
			User:      ctr.User,
			UTS:       ctr.Uts,
			Mounts:    ctr.Mounts,
		}
		psContainers = append(psContainers, psc)
	}
	return psContainers, nil
}
