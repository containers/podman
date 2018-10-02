package varlinkapi

import (
	"encoding/json"
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
		return call.ReplyPodNotFound(name)
	}
	opts := shared.PsOptions{}

	listPod, err := makeListPod(pod, opts)
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}

	return call.ReplyGetPod(listPod)
}

// InspectPod ...
func (i *LibpodAPI) InspectPod(call iopodman.VarlinkCall, name string) error {
	pod, err := i.Runtime.LookupPod(name)
	if err != nil {
		return call.ReplyPodNotFound(name)
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
		return call.ReplyPodNotFound(name)
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
func (i *LibpodAPI) StopPod(call iopodman.VarlinkCall, name string) error {
	pod, err := i.Runtime.LookupPod(name)
	if err != nil {
		return call.ReplyPodNotFound(name)
	}
	ctrErrs, err := pod.Stop(true)
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
		return call.ReplyPodNotFound(name)
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
		return call.ReplyPodNotFound(name)
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
		return call.ReplyPodNotFound(name)
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
		return call.ReplyPodNotFound(name)
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
		return call.ReplyPodNotFound(name)
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
		return call.ReplyPodNotFound(name)
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
