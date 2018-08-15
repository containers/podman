package varlinkapi

import (
	"encoding/json"
	"syscall"

	"github.com/projectatomic/libpod/cmd/podman/shared"
	"github.com/projectatomic/libpod/cmd/podman/varlink"
	"github.com/projectatomic/libpod/libpod"
)

// CreatePod ...
func (i *LibpodAPI) CreatePod(call iopodman.VarlinkCall, name, cgroupParent string, labels map[string]string) error {
	var options []libpod.PodCreateOption
	if cgroupParent != "" {
		options = append(options, libpod.WithPodCgroupParent(cgroupParent))
	}
	if len(labels) > 0 {
		options = append(options, libpod.WithPodLabels(labels))
	}
	if name != "" {
		options = append(options, libpod.WithPodName(name))
	}
	options = append(options, libpod.WithPodCgroups())

	pod, err := i.Runtime.NewPod(options...)
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
	ctrErrs, err := pod.Start(getContext())
	if err != nil && ctrErrs == nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	if ctrErrs != nil {
		containerErrs := make([]iopodman.PodContainerErrorData, len(ctrErrs))
		for ctr, reason := range ctrErrs {
			ctrErr := iopodman.PodContainerErrorData{Containerid: ctr, Reason: reason.Error()}
			containerErrs = append(containerErrs, ctrErr)
		}
		return call.ReplyPodContainerError(pod.ID(), containerErrs)
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
	if err != nil && ctrErrs == nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	if ctrErrs != nil {
		containerErrs := make([]iopodman.PodContainerErrorData, len(ctrErrs))
		for ctr, reason := range ctrErrs {
			ctrErr := iopodman.PodContainerErrorData{Containerid: ctr, Reason: reason.Error()}
			containerErrs = append(containerErrs, ctrErr)
		}
		return call.ReplyPodContainerError(pod.ID(), containerErrs)
	}

	return call.ReplyStopPod(pod.ID())
}

// RestartPod ...
func (i *LibpodAPI) RestartPod(call iopodman.VarlinkCall, name string) error {
	pod, err := i.Runtime.LookupPod(name)
	if err != nil {
		return call.ReplyPodNotFound(name)
	}
	ctrErrs, err := pod.Restart(getContext())
	if err != nil && ctrErrs == nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	if ctrErrs != nil {
		containerErrs := make([]iopodman.PodContainerErrorData, len(ctrErrs))
		for ctr, reason := range ctrErrs {
			ctrErr := iopodman.PodContainerErrorData{Containerid: ctr, Reason: reason.Error()}
			containerErrs = append(containerErrs, ctrErr)
		}
		return call.ReplyPodContainerError(pod.ID(), containerErrs)
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
	if err != nil && ctrErrs == nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	if ctrErrs != nil {
		containerErrs := make([]iopodman.PodContainerErrorData, len(ctrErrs))
		for ctr, reason := range ctrErrs {
			ctrErr := iopodman.PodContainerErrorData{Containerid: ctr, Reason: reason.Error()}
			containerErrs = append(containerErrs, ctrErr)
		}
		return call.ReplyPodContainerError(pod.ID(), containerErrs)
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
	if err != nil && ctrErrs == nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	if ctrErrs != nil {
		containerErrs := make([]iopodman.PodContainerErrorData, len(ctrErrs))
		for ctr, reason := range ctrErrs {
			ctrErr := iopodman.PodContainerErrorData{Containerid: ctr, Reason: reason.Error()}
			containerErrs = append(containerErrs, ctrErr)
		}
		return call.ReplyPodContainerError(pod.ID(), containerErrs)
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
	if err != nil && ctrErrs == nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	if ctrErrs != nil {
		containerErrs := make([]iopodman.PodContainerErrorData, len(ctrErrs))
		for ctr, reason := range ctrErrs {
			ctrErr := iopodman.PodContainerErrorData{Containerid: ctr, Reason: reason.Error()}
			containerErrs = append(containerErrs, ctrErr)
		}
		return call.ReplyPodContainerError(pod.ID(), containerErrs)
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
