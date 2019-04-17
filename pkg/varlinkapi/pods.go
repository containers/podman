// +build varlink

package varlinkapi

import (
	"encoding/json"
	"fmt"
	"github.com/containers/libpod/pkg/adapter/shortcuts"
	"github.com/containers/libpod/pkg/rootless"
	"syscall"

	"github.com/containers/libpod/cmd/podman/shared"
	"github.com/containers/libpod/cmd/podman/varlink"
	"github.com/containers/libpod/libpod"
)

// CreatePod ...
func (i *LibpodAPI) CreatePod(call iopodman.VarlinkCall, create iopodman.PodCreate) error {
	var options []libpod.PodCreateOption
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
		if rootless.IsRootless() {
			return call.ReplyErrorOccurred("rootless networking does not allow port binding to the host")
		}
		portBindings, err := shared.CreatePortBindings(create.Publish)
		if err != nil {
			return err
		}
		options = append(options, libpod.WithInfraContainerPorts(portBindings))

	}
	if create.Infra {
		options = append(options, libpod.WithInfraContainer())
		nsOptions, err := shared.GetNamespaceOptions(create.Share)
		if err != nil {
			return err
		}
		options = append(options, nsOptions...)
	}
	options = append(options, libpod.WithPodCgroups())

	pod, err := i.Runtime.NewPod(getContext(), options...)
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	return call.ReplyCreatePod(pod.ID())
}

// ListPods ...
func (i *LibpodAPI) ListPods(call iopodman.VarlinkCall) error {
	var (
		listPods []iopodman.ListPodData
	)

	pods, err := i.Runtime.GetAllPods()
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	opts := shared.PsOptions{}
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
func (i *LibpodAPI) GetPod(call iopodman.VarlinkCall, name string) error {
	pod, err := i.Runtime.LookupPod(name)
	if err != nil {
		return call.ReplyPodNotFound(name, err.Error())
	}
	opts := shared.PsOptions{}

	listPod, err := makeListPod(pod, opts)
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}

	return call.ReplyGetPod(listPod)
}

// GetPodsByStatus returns a slice of pods filtered by a libpod status
func (i *LibpodAPI) GetPodsByStatus(call iopodman.VarlinkCall, statuses []string) error {
	filterFuncs := func(p *libpod.Pod) bool {
		state, _ := shared.GetPodStatus(p)
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
func (i *LibpodAPI) InspectPod(call iopodman.VarlinkCall, name string) error {
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
func (i *LibpodAPI) StartPod(call iopodman.VarlinkCall, name string) error {
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
func (i *LibpodAPI) StopPod(call iopodman.VarlinkCall, name string, timeout int64) error {
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
func (i *LibpodAPI) RestartPod(call iopodman.VarlinkCall, name string) error {
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
func (i *LibpodAPI) KillPod(call iopodman.VarlinkCall, name string, signal int64) error {
	killSignal := uint(syscall.SIGTERM)
	if signal != -1 {
		killSignal = uint(signal)
	}

	pod, err := i.Runtime.LookupPod(name)
	if err != nil {
		return call.ReplyPodNotFound(name, err.Error())
	}
	ctrErrs, err := pod.Kill(killSignal)
	callErr := handlePodCall(call, pod, ctrErrs, err)
	if callErr != nil {
		return err
	}
	return call.ReplyKillPod(pod.ID())
}

// PausePod ...
func (i *LibpodAPI) PausePod(call iopodman.VarlinkCall, name string) error {
	pod, err := i.Runtime.LookupPod(name)
	if err != nil {
		return call.ReplyPodNotFound(name, err.Error())
	}
	ctrErrs, err := pod.Pause()
	callErr := handlePodCall(call, pod, ctrErrs, err)
	if callErr != nil {
		return err
	}
	return call.ReplyPausePod(pod.ID())
}

// UnpausePod ...
func (i *LibpodAPI) UnpausePod(call iopodman.VarlinkCall, name string) error {
	pod, err := i.Runtime.LookupPod(name)
	if err != nil {
		return call.ReplyPodNotFound(name, err.Error())
	}
	ctrErrs, err := pod.Unpause()
	callErr := handlePodCall(call, pod, ctrErrs, err)
	if callErr != nil {
		return err
	}
	return call.ReplyUnpausePod(pod.ID())
}

// RemovePod ...
func (i *LibpodAPI) RemovePod(call iopodman.VarlinkCall, name string, force bool) error {
	ctx := getContext()
	pod, err := i.Runtime.LookupPod(name)
	if err != nil {
		return call.ReplyPodNotFound(name, err.Error())
	}
	if err = i.Runtime.RemovePod(ctx, pod, force, force); err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}

	return call.ReplyRemovePod(pod.ID())
}

// GetPodStats ...
func (i *LibpodAPI) GetPodStats(call iopodman.VarlinkCall, name string) error {
	pod, err := i.Runtime.LookupPod(name)
	if err != nil {
		return call.ReplyPodNotFound(name, err.Error())
	}
	prevStats := make(map[string]*libpod.ContainerStats)
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

// GetPodsByContext returns a slice of pod ids based on all, latest, or a list
func (i *LibpodAPI) GetPodsByContext(call iopodman.VarlinkCall, all, latest bool, input []string) error {
	var podids []string

	pods, err := shortcuts.GetPodsByContext(all, latest, input, i.Runtime)
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	for _, p := range pods {
		podids = append(podids, p.ID())
	}
	return call.ReplyGetPodsByContext(podids)
}

// PodStateData returns a container's state data in string format
func (i *LibpodAPI) PodStateData(call iopodman.VarlinkCall, name string) error {
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
func (i *LibpodAPI) TopPod(call iopodman.VarlinkCall, name string, latest bool, descriptors []string) error {
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

	podStatus, err := shared.GetPodStatus(pod)
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
