// +build remoteclient

package adapter

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/shared"
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

// CreatePod creates a pod for the remote client over a varlink connection
func (r *LocalRuntime) CreatePod(ctx context.Context, cli *cliconfig.PodCreateValues, labels map[string]string) (string, error) {
	pc := iopodman.PodCreate{
		Name:         cli.Name,
		CgroupParent: cli.CgroupParent,
		Labels:       labels,
		Share:        strings.Split(cli.Share, ","),
		Infra:        cli.Infra,
		InfraCommand: cli.InfraCommand,
		InfraImage:   cli.InfraCommand,
		Publish:      cli.Publish,
	}

	return iopodman.CreatePod().Call(r.Conn, pc)
}

// GetAllPods is a helper function that gets all pods for the remote client
func (r *LocalRuntime) GetAllPods() ([]*Pod, error) {
	var pods []*Pod
	podIDs, err := iopodman.GetPodsByContext().Call(r.Conn, true, false, []string{})
	if err != nil {
		return nil, err
	}
	for _, p := range podIDs {
		pod, err := r.LookupPod(p)
		if err != nil {
			return nil, err
		}
		pods = append(pods, pod)
	}
	return pods, nil
}

// ID returns the id of a remote pod
func (p *Pod) ID() string {
	return p.config.ID
}

// Name returns the name of the remote pod
func (p *Pod) Name() string {
	return p.config.Name
}

// AllContainersByID returns a slice of a pod's container IDs
func (p *Pod) AllContainersByID() ([]string, error) {
	var containerIDs []string
	for _, ctr := range p.containers {
		containerIDs = append(containerIDs, ctr.ID)
	}
	return containerIDs, nil
}

// AllContainers returns a pods containers
func (p *Pod) AllContainers() ([]*Container, error) {
	var containers []*Container
	for _, ctr := range p.containers {
		container, err := p.Runtime.LookupContainer(ctr.ID)
		if err != nil {
			return nil, err
		}
		containers = append(containers, container)
	}
	return containers, nil
}

// Status ...
func (p *Pod) Status() (map[string]libpod.ContainerStatus, error) {
	ctrs := make(map[string]libpod.ContainerStatus)
	for _, i := range p.containers {
		var status libpod.ContainerStatus
		switch i.State {
		case "exited":
			status = libpod.ContainerStateExited
		case "stopped":
			status = libpod.ContainerStateStopped
		case "running":
			status = libpod.ContainerStateRunning
		case "paused":
			status = libpod.ContainerStatePaused
		case "created":
			status = libpod.ContainerStateCreated
		case "configured":
			status = libpod.ContainerStateConfigured
		default:
			status = libpod.ContainerStateUnknown
		}
		ctrs[i.ID] = status
	}
	return ctrs, nil
}

// GetPodStatus is a wrapper to get the string version of the status
func (p *Pod) GetPodStatus() (string, error) {
	ctrStatuses, err := p.Status()
	if err != nil {
		return "", err
	}
	return shared.CreatePodStatusResults(ctrStatuses)
}

// InfraContainerID returns the ID of the infra container in a pod
func (p *Pod) InfraContainerID() (string, error) {
	return p.state.InfraContainerID, nil
}

// CreatedTime returns the time the container was created as a time.Time
func (p *Pod) CreatedTime() time.Time {
	return p.config.CreatedTime
}

// SharesPID ....
func (p *Pod) SharesPID() bool {
	return p.config.UsePodPID
}

// SharesIPC returns whether containers in pod
// default to use IPC namespace of first container in pod
func (p *Pod) SharesIPC() bool {
	return p.config.UsePodIPC
}

// SharesNet returns whether containers in pod
// default to use network namespace of first container in pod
func (p *Pod) SharesNet() bool {
	return p.config.UsePodNet
}

// SharesMount returns whether containers in pod
// default to use PID namespace of first container in pod
func (p *Pod) SharesMount() bool {
	return p.config.UsePodMount
}

// SharesUser returns whether containers in pod
// default to use user namespace of first container in pod
func (p *Pod) SharesUser() bool {
	return p.config.UsePodUser
}

// SharesUTS returns whether containers in pod
// default to use UTS namespace of first container in pod
func (p *Pod) SharesUTS() bool {
	return p.config.UsePodUTS
}

// SharesCgroup returns whether containers in the pod will default to this pod's
// cgroup instead of the default libpod parent
func (p *Pod) SharesCgroup() bool {
	return p.config.UsePodCgroup
}

// CgroupParent returns the pod's CGroup parent
func (p *Pod) CgroupParent() string {
	return p.config.CgroupParent
}
