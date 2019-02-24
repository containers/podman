// +build remoteclient

package adapter

import (
	"context"
	"encoding/json"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/varlink"
	"github.com/containers/libpod/libpod"
	"github.com/pkg/errors"
	"github.com/ulule/deepcopier"
)

// Pod ...
type Pod struct {
	remotepod
}

type remotepod struct {
	config     *libpod.PodConfig
	state      *libpod.PodInspectState
	containers []libpod.PodContainerInfo
	Runtime    *LocalRuntime
}

// RemovePods removes one or more based on the cli context.
func (r *LocalRuntime) RemovePods(ctx context.Context, cli *cliconfig.PodRmValues) ([]string, []error) {
	var (
		rmErrs []error
		rmPods []string
	)
	podIDs, err := iopodman.GetPodsByContext().Call(r.Conn, cli.All, cli.Latest, cli.InputArgs)
	if err != nil {
		rmErrs = append(rmErrs, err)
		return nil, rmErrs
	}

	for _, p := range podIDs {
		reply, err := iopodman.RemovePod().Call(r.Conn, p, cli.Force)
		if err != nil {
			rmErrs = append(rmErrs, err)
		} else {
			rmPods = append(rmPods, reply)
		}
	}
	return rmPods, rmErrs
}

// Inspect looks up a pod by name or id and embeds its data into a remote pod
// object.
func (r *LocalRuntime) Inspect(nameOrID string) (*Pod, error) {
	reply, err := iopodman.PodStateData().Call(r.Conn, nameOrID)
	if err != nil {
		return nil, err
	}
	data := libpod.PodInspect{}
	if err := json.Unmarshal([]byte(reply), &data); err != nil {
		return nil, err
	}
	pod := Pod{}
	pod.Runtime = r
	pod.config = data.Config
	pod.state = data.State
	pod.containers = data.Containers
	return &pod, nil
}

// GetLatestPod gets the latest pod and wraps it in an adapter pod
func (r *LocalRuntime) GetLatestPod() (*Pod, error) {
	reply, err := iopodman.GetPodsByContext().Call(r.Conn, false, true, nil)
	if err != nil {
		return nil, err
	}
	if len(reply) > 0 {
		return r.Inspect(reply[0])
	}
	return nil, errors.New("no pods exist")
}

// LookupPod gets a pod by name or ID and wraps it in an adapter pod
func (r *LocalRuntime) LookupPod(nameOrID string) (*Pod, error) {
	return r.Inspect(nameOrID)
}

// Inspect, like libpod pod inspect, returns a libpod.PodInspect object from
// the data of a remotepod data struct
func (p *Pod) Inspect() (*libpod.PodInspect, error) {
	config := new(libpod.PodConfig)
	deepcopier.Copy(p.remotepod.config).To(config)
	inspectData := libpod.PodInspect{
		Config:     config,
		State:      p.remotepod.state,
		Containers: p.containers,
	}
	return &inspectData, nil
}

// StopPods stops pods based on the cli context from the remote client.
func (r *LocalRuntime) StopPods(ctx context.Context, cli *cliconfig.PodStopValues) ([]string, []error) {
	var (
		stopErrs []error
		stopPods []string
	)
	var timeout int64 = -1
	if cli.Flags().Changed("timeout") {
		timeout = int64(cli.Timeout)
	}
	podIDs, err := iopodman.GetPodsByContext().Call(r.Conn, cli.All, cli.Latest, cli.InputArgs)
	if err != nil {
		return nil, []error{err}
	}

	for _, p := range podIDs {
		podID, err := iopodman.StopPod().Call(r.Conn, p, timeout)
		if err != nil {
			stopErrs = append(stopErrs, err)
		} else {
			stopPods = append(stopPods, podID)
		}
	}
	return stopPods, stopErrs
}

// KillPods kills pods over varlink for the remoteclient
func (r *LocalRuntime) KillPods(ctx context.Context, cli *cliconfig.PodKillValues, signal uint) ([]string, []error) {
	var (
		killErrs []error
		killPods []string
	)

	podIDs, err := iopodman.GetPodsByContext().Call(r.Conn, cli.All, cli.Latest, cli.InputArgs)
	if err != nil {
		return nil, []error{err}
	}

	for _, p := range podIDs {
		podID, err := iopodman.KillPod().Call(r.Conn, p, int64(signal))
		if err != nil {
			killErrs = append(killErrs, err)
		} else {
			killPods = append(killPods, podID)
		}
	}
	return killPods, killErrs
}

// StartPods starts pods for the remote client over varlink
func (r *LocalRuntime) StartPods(ctx context.Context, cli *cliconfig.PodStartValues) ([]string, []error) {
	var (
		startErrs []error
		startPods []string
	)

	podIDs, err := iopodman.GetPodsByContext().Call(r.Conn, cli.All, cli.Latest, cli.InputArgs)
	if err != nil {
		return nil, []error{err}
	}

	for _, p := range podIDs {
		podID, err := iopodman.StartPod().Call(r.Conn, p)
		if err != nil {
			startErrs = append(startErrs, err)
		} else {
			startPods = append(startPods, podID)
		}
	}
	return startPods, startErrs
}
