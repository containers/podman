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
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/varlinkapi"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// PodContainerStats is struct containing an adapter Pod and a libpod
// ContainerStats and is used primarily for outputting pod stats.
type PodContainerStats struct {
	Pod            *Pod
	ContainerStats map[string]*libpod.ContainerStats
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
	if err := libpod.JSONDeepCopy(p.remotepod.config, config); err != nil {
		return nil, err
	}
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
	var share []string
	if cli.Share != "" {
		share = strings.Split(cli.Share, ",")
	}
	pc := iopodman.PodCreate{
		Name:         cli.Name,
		CgroupParent: cli.CgroupParent,
		Labels:       labels,
		Share:        share,
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

// GetPodsByStatus returns a slice of pods filtered by a libpod status
func (r *LocalRuntime) GetPodsByStatus(statuses []string) ([]*Pod, error) {
	podIDs, err := iopodman.GetPodsByStatus().Call(r.Conn, statuses)
	if err != nil {
		return nil, err
	}
	pods := make([]*Pod, 0, len(podIDs))
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
func (p *Pod) Status() (map[string]define.ContainerStatus, error) {
	ctrs := make(map[string]define.ContainerStatus)
	for _, i := range p.containers {
		var status define.ContainerStatus
		switch i.State {
		case "exited":
			status = define.ContainerStateExited
		case "stopped":
			status = define.ContainerStateStopped
		case "running":
			status = define.ContainerStateRunning
		case "paused":
			status = define.ContainerStatePaused
		case "created":
			status = define.ContainerStateCreated
		case "define.red":
			status = define.ContainerStateConfigured
		default:
			status = define.ContainerStateUnknown
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

// PausePods pauses a pod using varlink and the remote client
func (r *LocalRuntime) PausePods(c *cliconfig.PodPauseValues) ([]string, map[string]error, []error) {
	var (
		pauseIDs    []string
		pauseErrors []error
	)
	containerErrors := make(map[string]error)

	pods, err := iopodman.GetPodsByContext().Call(r.Conn, c.All, c.Latest, c.InputArgs)
	if err != nil {
		pauseErrors = append(pauseErrors, err)
		return nil, containerErrors, pauseErrors
	}
	for _, pod := range pods {
		reply, err := iopodman.PausePod().Call(r.Conn, pod)
		if err != nil {
			pauseErrors = append(pauseErrors, err)
			continue
		}
		pauseIDs = append(pauseIDs, reply)
	}
	return pauseIDs, nil, pauseErrors
}

// UnpausePods unpauses a pod using varlink and the remote client
func (r *LocalRuntime) UnpausePods(c *cliconfig.PodUnpauseValues) ([]string, map[string]error, []error) {
	var (
		unpauseIDs    []string
		unpauseErrors []error
	)
	containerErrors := make(map[string]error)

	pods, err := iopodman.GetPodsByContext().Call(r.Conn, c.All, c.Latest, c.InputArgs)
	if err != nil {
		unpauseErrors = append(unpauseErrors, err)
		return nil, containerErrors, unpauseErrors
	}
	for _, pod := range pods {
		reply, err := iopodman.UnpausePod().Call(r.Conn, pod)
		if err != nil {
			unpauseErrors = append(unpauseErrors, err)
			continue
		}
		unpauseIDs = append(unpauseIDs, reply)
	}
	return unpauseIDs, nil, unpauseErrors
}

// RestartPods restarts pods using varlink and the remote client
func (r *LocalRuntime) RestartPods(ctx context.Context, c *cliconfig.PodRestartValues) ([]string, map[string]error, []error) {
	var (
		restartIDs    []string
		restartErrors []error
	)
	containerErrors := make(map[string]error)

	pods, err := iopodman.GetPodsByContext().Call(r.Conn, c.All, c.Latest, c.InputArgs)
	if err != nil {
		restartErrors = append(restartErrors, err)
		return nil, containerErrors, restartErrors
	}
	for _, pod := range pods {
		reply, err := iopodman.RestartPod().Call(r.Conn, pod)
		if err != nil {
			restartErrors = append(restartErrors, err)
			continue
		}
		restartIDs = append(restartIDs, reply)
	}
	return restartIDs, nil, restartErrors
}

// PodTop gets top statistics for a pod
func (r *LocalRuntime) PodTop(c *cliconfig.PodTopValues, descriptors []string) ([]string, error) {
	var (
		latest  bool
		podName string
	)
	if c.Latest {
		latest = true
	} else {
		podName = c.InputArgs[0]
	}
	return iopodman.TopPod().Call(r.Conn, podName, latest, descriptors)
}

// GetStatPods returns pods for use in pod stats
func (r *LocalRuntime) GetStatPods(c *cliconfig.PodStatsValues) ([]*Pod, error) {
	var (
		pods    []*Pod
		err     error
		podIDs  []string
		running bool
	)

	if len(c.InputArgs) > 0 || c.Latest || c.All {
		podIDs, err = iopodman.GetPodsByContext().Call(r.Conn, c.All, c.Latest, c.InputArgs)
	} else {
		podIDs, err = iopodman.GetPodsByContext().Call(r.Conn, true, false, []string{})
		running = true
	}
	if err != nil {
		return nil, err
	}
	for _, p := range podIDs {
		pod, err := r.Inspect(p)
		if err != nil {
			return nil, err
		}
		if running {
			status, err := pod.GetPodStatus()
			if err != nil {
				// if we cannot get the status of the pod, skip and move on
				continue
			}
			if strings.ToUpper(status) != "RUNNING" {
				// if the pod is not running, skip and move on as well
				continue
			}
		}
		pods = append(pods, pod)
	}
	return pods, nil
}

// GetPodStats returns the stats for each of its containers
func (p *Pod) GetPodStats(previousContainerStats map[string]*libpod.ContainerStats) (map[string]*libpod.ContainerStats, error) {
	var (
		ok       bool
		prevStat *libpod.ContainerStats
	)
	newContainerStats := make(map[string]*libpod.ContainerStats)
	containers, err := p.AllContainers()
	if err != nil {
		return nil, err
	}
	for _, c := range containers {
		if prevStat, ok = previousContainerStats[c.ID()]; !ok {
			prevStat = &libpod.ContainerStats{ContainerID: c.ID()}
		}
		cStats := iopodman.ContainerStats{
			Id:           prevStat.ContainerID,
			Name:         prevStat.Name,
			Cpu:          prevStat.CPU,
			Cpu_nano:     int64(prevStat.CPUNano),
			System_nano:  int64(prevStat.SystemNano),
			Mem_usage:    int64(prevStat.MemUsage),
			Mem_limit:    int64(prevStat.MemLimit),
			Mem_perc:     prevStat.MemPerc,
			Net_input:    int64(prevStat.NetInput),
			Net_output:   int64(prevStat.NetOutput),
			Block_input:  int64(prevStat.BlockInput),
			Block_output: int64(prevStat.BlockOutput),
			Pids:         int64(prevStat.PIDs),
		}
		stats, err := iopodman.GetContainerStatsWithHistory().Call(p.Runtime.Conn, cStats)
		if err != nil {
			return nil, err
		}
		newStats := varlinkapi.ContainerStatsToLibpodContainerStats(stats)
		// If the container wasn't running, don't include it
		// but also suppress the error
		if err != nil && errors.Cause(err) != define.ErrCtrStateInvalid {
			return nil, err
		}
		if err == nil {
			newContainerStats[c.ID()] = &newStats
		}
	}
	return newContainerStats, nil
}

// RemovePod removes a pod
// If removeCtrs is specified, containers will be removed
// Otherwise, a pod that is not empty will return an error and not be removed
// If force is specified with removeCtrs, all containers will be stopped before
// being removed
// Otherwise, the pod will not be removed if any containers are running
func (r *LocalRuntime) RemovePod(ctx context.Context, p *Pod, removeCtrs, force bool) error {
	_, err := iopodman.RemovePod().Call(r.Conn, p.ID(), force)
	if err != nil {
		return err
	}
	return nil
}

// PrunePods...
func (r *LocalRuntime) PrunePods(ctx context.Context, cli *cliconfig.PodPruneValues) ([]string, map[string]error, error) {
	var (
		ok       = []string{}
		failures = map[string]error{}
	)
	states := []string{define.PodStateStopped, define.PodStateExited}
	if cli.Force {
		states = append(states, define.PodStateRunning)
	}

	ids, err := iopodman.GetPodsByStatus().Call(r.Conn, states)
	if err != nil {
		return ok, failures, err
	}
	if len(ids) < 1 {
		return ok, failures, nil
	}

	for _, id := range ids {
		_, err := iopodman.RemovePod().Call(r.Conn, id, cli.Force)
		if err != nil {
			logrus.Debugf("Failed to remove pod %s: %s", id, err.Error())
			failures[id] = err
		} else {
			ok = append(ok, id)
		}
	}
	return ok, failures, nil
}

// PlayKubeYAML creates pods and containers from a kube YAML file
func (r *LocalRuntime) PlayKubeYAML(ctx context.Context, c *cliconfig.KubePlayValues, yamlFile string) (*Pod, error) {
	return nil, define.ErrNotImplemented
}
