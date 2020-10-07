// +build varlink

package varlinkapi

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"syscall"

	"github.com/containers/podman/v2/libpod"
	"github.com/containers/podman/v2/libpod/define"
	iopodman "github.com/containers/podman/v2/pkg/varlink"
	"github.com/cri-o/ocicni/pkg/ocicni"
	"github.com/docker/go-connections/nat"
	"github.com/pkg/errors"
)

// CreatePod ...
func (i *VarlinkAPI) CreatePod(call iopodman.VarlinkCall, create iopodman.PodCreate) error {
	var options []libpod.PodCreateOption
	if create.Infra {
		options = append(options, libpod.WithInfraContainer())
		nsOptions, err := GetNamespaceOptions(create.Share)
		if err != nil {
			return err
		}
		options = append(options, nsOptions...)
	}
	if create.CgroupParent != "" {
		options = append(options, libpod.WithPodCgroupParent(create.CgroupParent))
	}
	if len(create.Labels) > 0 {
		options = append(options, libpod.WithPodLabels(create.Labels))
	}
	if create.Name != "" {
		options = append(options, libpod.WithPodName(create.Name))
	}
	if len(create.Share) > 0 && !create.Infra {
		return call.ReplyErrorOccurred("You cannot share kernel namespaces on the pod level without an infra container")
	}
	if len(create.Share) == 0 && create.Infra {
		return call.ReplyErrorOccurred("You must share kernel namespaces to run an infra container")
	}

	if len(create.Publish) > 0 {
		if !create.Infra {
			return call.ReplyErrorOccurred("you must have an infra container to publish port bindings to the host")
		}
		portBindings, err := CreatePortBindings(create.Publish)
		if err != nil {
			return call.ReplyErrorOccurred(err.Error())
		}
		options = append(options, libpod.WithInfraContainerPorts(portBindings))

	}
	options = append(options, libpod.WithPodCgroups())

	pod, err := i.Runtime.NewPod(getContext(), options...)
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	return call.ReplyCreatePod(pod.ID())
}

// ListPods ...
func (i *VarlinkAPI) ListPods(call iopodman.VarlinkCall) error {
	var (
		listPods []iopodman.ListPodData
	)

	pods, err := i.Runtime.GetAllPods()
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	opts := PsOptions{}
	for _, pod := range pods {
		listPod, err := makeListPod(pod, opts)
		if err != nil {
			return call.ReplyErrorOccurred(err.Error())
		}
		listPods = append(listPods, listPod)
	}
	return call.ReplyListPods(listPods)
}

// GetPod ...
func (i *VarlinkAPI) GetPod(call iopodman.VarlinkCall, name string) error {
	pod, err := i.Runtime.LookupPod(name)
	if err != nil {
		return call.ReplyPodNotFound(name, err.Error())
	}
	opts := PsOptions{}

	listPod, err := makeListPod(pod, opts)
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}

	return call.ReplyGetPod(listPod)
}

// GetPodsByStatus returns a slice of pods filtered by a libpod status
func (i *VarlinkAPI) GetPodsByStatus(call iopodman.VarlinkCall, statuses []string) error {
	filterFuncs := func(p *libpod.Pod) bool {
		state, _ := p.GetPodStatus()
		for _, status := range statuses {
			if state == status {
				return true
			}
		}
		return false
	}
	filteredPods, err := i.Runtime.Pods(filterFuncs)
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	podIDs := make([]string, 0, len(filteredPods))
	for _, p := range filteredPods {
		podIDs = append(podIDs, p.ID())
	}
	return call.ReplyGetPodsByStatus(podIDs)
}

// InspectPod ...
func (i *VarlinkAPI) InspectPod(call iopodman.VarlinkCall, name string) error {
	pod, err := i.Runtime.LookupPod(name)
	if err != nil {
		return call.ReplyPodNotFound(name, err.Error())
	}
	inspectData, err := pod.Inspect()
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	b, err := json.Marshal(&inspectData)
	if err != nil {
		return call.ReplyErrorOccurred("unable to serialize")
	}
	return call.ReplyInspectPod(string(b))
}

// StartPod ...
func (i *VarlinkAPI) StartPod(call iopodman.VarlinkCall, name string) error {
	pod, err := i.Runtime.LookupPod(name)
	if err != nil {
		return call.ReplyPodNotFound(name, err.Error())
	}
	ctnrs, err := pod.AllContainers()
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	if 0 == len(ctnrs) {
		return call.ReplyNoContainersInPod(name)
	}
	ctrErrs, err := pod.Start(getContext())
	callErr := handlePodCall(call, pod, ctrErrs, err)
	if callErr != nil {
		return err
	}
	return call.ReplyStartPod(pod.ID())
}

// StopPod ...
func (i *VarlinkAPI) StopPod(call iopodman.VarlinkCall, name string, timeout int64) error {
	pod, err := i.Runtime.LookupPod(name)
	if err != nil {
		return call.ReplyPodNotFound(name, err.Error())
	}
	ctrErrs, err := pod.StopWithTimeout(getContext(), true, int(timeout))
	callErr := handlePodCall(call, pod, ctrErrs, err)
	if callErr != nil {
		return err
	}
	return call.ReplyStopPod(pod.ID())
}

// RestartPod ...
func (i *VarlinkAPI) RestartPod(call iopodman.VarlinkCall, name string) error {
	pod, err := i.Runtime.LookupPod(name)
	if err != nil {
		return call.ReplyPodNotFound(name, err.Error())
	}
	ctnrs, err := pod.AllContainers()
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	if 0 == len(ctnrs) {
		return call.ReplyNoContainersInPod(name)
	}
	ctrErrs, err := pod.Restart(getContext())
	callErr := handlePodCall(call, pod, ctrErrs, err)
	if callErr != nil {
		return err
	}
	return call.ReplyRestartPod(pod.ID())
}

// KillPod kills the running containers in a pod.  If you want to use the default SIGTERM signal,
// just send a -1 for the signal arg.
func (i *VarlinkAPI) KillPod(call iopodman.VarlinkCall, name string, signal int64) error {
	killSignal := uint(syscall.SIGTERM)
	if signal != -1 {
		killSignal = uint(signal)
	}

	pod, err := i.Runtime.LookupPod(name)
	if err != nil {
		return call.ReplyPodNotFound(name, err.Error())
	}
	ctrErrs, err := pod.Kill(context.TODO(), killSignal)
	callErr := handlePodCall(call, pod, ctrErrs, err)
	if callErr != nil {
		return err
	}
	return call.ReplyKillPod(pod.ID())
}

// PausePod ...
func (i *VarlinkAPI) PausePod(call iopodman.VarlinkCall, name string) error {
	pod, err := i.Runtime.LookupPod(name)
	if err != nil {
		return call.ReplyPodNotFound(name, err.Error())
	}
	ctrErrs, err := pod.Pause(context.TODO())
	callErr := handlePodCall(call, pod, ctrErrs, err)
	if callErr != nil {
		return err
	}
	return call.ReplyPausePod(pod.ID())
}

// UnpausePod ...
func (i *VarlinkAPI) UnpausePod(call iopodman.VarlinkCall, name string) error {
	pod, err := i.Runtime.LookupPod(name)
	if err != nil {
		return call.ReplyPodNotFound(name, err.Error())
	}
	ctrErrs, err := pod.Unpause(context.TODO())
	callErr := handlePodCall(call, pod, ctrErrs, err)
	if callErr != nil {
		return err
	}
	return call.ReplyUnpausePod(pod.ID())
}

// RemovePod ...
func (i *VarlinkAPI) RemovePod(call iopodman.VarlinkCall, name string, force bool) error {
	ctx := getContext()
	pod, err := i.Runtime.LookupPod(name)
	if err != nil {
		return call.ReplyPodNotFound(name, err.Error())
	}
	if err = i.Runtime.RemovePod(ctx, pod, true, force); err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}

	return call.ReplyRemovePod(pod.ID())
}

// GetPodStats ...
func (i *VarlinkAPI) GetPodStats(call iopodman.VarlinkCall, name string) error {
	pod, err := i.Runtime.LookupPod(name)
	if err != nil {
		return call.ReplyPodNotFound(name, err.Error())
	}
	prevStats := make(map[string]*define.ContainerStats)
	podStats, err := pod.GetPodStats(prevStats)
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	if len(podStats) == 0 {
		return call.ReplyNoContainerRunning()
	}
	containersStats := make([]iopodman.ContainerStats, 0)
	for ctrID, containerStats := range podStats {
		cs := iopodman.ContainerStats{
			Id:           ctrID,
			Name:         containerStats.Name,
			Cpu:          containerStats.CPU,
			Cpu_nano:     int64(containerStats.CPUNano),
			System_nano:  int64(containerStats.SystemNano),
			Mem_usage:    int64(containerStats.MemUsage),
			Mem_limit:    int64(containerStats.MemLimit),
			Mem_perc:     containerStats.MemPerc,
			Net_input:    int64(containerStats.NetInput),
			Net_output:   int64(containerStats.NetOutput),
			Block_input:  int64(containerStats.BlockInput),
			Block_output: int64(containerStats.BlockOutput),
			Pids:         int64(containerStats.PIDs),
		}
		containersStats = append(containersStats, cs)
	}
	return call.ReplyGetPodStats(pod.ID(), containersStats)
}

// getPodsByContext returns a slice of pod ids based on all, latest, or a list
func (i *VarlinkAPI) GetPodsByContext(call iopodman.VarlinkCall, all, latest bool, input []string) error {
	var podids []string

	pods, err := getPodsByContext(all, latest, input, i.Runtime)
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	for _, p := range pods {
		podids = append(podids, p.ID())
	}
	return call.ReplyGetPodsByContext(podids)
}

// PodStateData returns a container's state data in string format
func (i *VarlinkAPI) PodStateData(call iopodman.VarlinkCall, name string) error {
	pod, err := i.Runtime.LookupPod(name)
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	data, err := pod.Inspect()
	if err != nil {
		return call.ReplyErrorOccurred("unable to obtain pod state")
	}
	b, err := json.Marshal(data)
	if err != nil {
		return call.ReplyErrorOccurred("unable to serialize pod inspect data")
	}
	return call.ReplyPodStateData(string(b))
}

// TopPod provides the top stats for a given or latest pod
func (i *VarlinkAPI) TopPod(call iopodman.VarlinkCall, name string, latest bool, descriptors []string) error {
	var (
		pod *libpod.Pod
		err error
	)
	if latest {
		name = "latest"
		pod, err = i.Runtime.GetLatestPod()
	} else {
		pod, err = i.Runtime.LookupPod(name)
	}
	if err != nil {
		return call.ReplyPodNotFound(name, err.Error())
	}

	podStatus, err := pod.GetPodStatus()
	if err != nil {
		return call.ReplyErrorOccurred(fmt.Sprintf("unable to get status for pod %s", pod.ID()))
	}
	if podStatus != "Running" {
		return call.ReplyErrorOccurred("pod top can only be used on pods with at least one running container")
	}
	reply, err := pod.GetPodPidInformation(descriptors)
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	return call.ReplyTopPod(reply)
}

// CreatePortBindings iterates ports mappings and exposed ports into a format CNI understands
func CreatePortBindings(ports []string) ([]ocicni.PortMapping, error) {
	var portBindings []ocicni.PortMapping
	// The conversion from []string to natBindings is temporary while mheon reworks the port
	// deduplication code.  Eventually that step will not be required.
	_, natBindings, err := nat.ParsePortSpecs(ports)
	if err != nil {
		return nil, err
	}
	for containerPb, hostPb := range natBindings {
		var pm ocicni.PortMapping
		pm.ContainerPort = int32(containerPb.Int())
		for _, i := range hostPb {
			var hostPort int
			var err error
			pm.HostIP = i.HostIP
			if i.HostPort == "" {
				hostPort = containerPb.Int()
			} else {
				hostPort, err = strconv.Atoi(i.HostPort)
				if err != nil {
					return nil, errors.Wrapf(err, "unable to convert host port to integer")
				}
			}

			pm.HostPort = int32(hostPort)
			pm.Protocol = containerPb.Proto()
			portBindings = append(portBindings, pm)
		}
	}
	return portBindings, nil
}
