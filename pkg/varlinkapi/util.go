// +build varlink

package varlinkapi

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/containers/buildah"
	"github.com/containers/podman/v2/libpod"
	"github.com/containers/podman/v2/libpod/define"
	"github.com/containers/podman/v2/pkg/channel"
	iopodman "github.com/containers/podman/v2/pkg/varlink"
	"github.com/containers/storage/pkg/archive"
)

// getContext returns a non-nil, empty context
func getContext() context.Context {
	return context.TODO()
}

func makeListContainer(containerID string, batchInfo BatchContainerStruct) iopodman.Container {
	var (
		mounts []iopodman.ContainerMount
		ports  []iopodman.ContainerPortMappings
	)
	ns := GetNamespaces(batchInfo.Pid)

	for _, mount := range batchInfo.ConConfig.Spec.Mounts {
		m := iopodman.ContainerMount{
			Destination: mount.Destination,
			Type:        mount.Type,
			Source:      mount.Source,
			Options:     mount.Options,
		}
		mounts = append(mounts, m)
	}

	for _, pm := range batchInfo.ConConfig.PortMappings {
		p := iopodman.ContainerPortMappings{
			Host_port:      strconv.Itoa(int(pm.HostPort)),
			Host_ip:        pm.HostIP,
			Protocol:       pm.Protocol,
			Container_port: strconv.Itoa(int(pm.ContainerPort)),
		}
		ports = append(ports, p)

	}

	// If we find this needs to be done for other container endpoints, we should
	// convert this to a separate function or a generic map from struct function.
	namespace := iopodman.ContainerNameSpace{
		User:   ns.User,
		Uts:    ns.UTS,
		Pidns:  ns.PIDNS,
		Pid:    ns.PID,
		Cgroup: ns.Cgroup,
		Net:    ns.NET,
		Mnt:    ns.MNT,
		Ipc:    ns.IPC,
	}

	lc := iopodman.Container{
		Id:               containerID,
		Image:            batchInfo.ConConfig.RootfsImageName,
		Imageid:          batchInfo.ConConfig.RootfsImageID,
		Command:          batchInfo.ConConfig.Spec.Process.Args,
		Createdat:        batchInfo.ConConfig.CreatedTime.Format(time.RFC3339),
		Runningfor:       time.Since(batchInfo.ConConfig.CreatedTime).String(),
		Status:           batchInfo.ConState.String(),
		Ports:            ports,
		Names:            batchInfo.ConConfig.Name,
		Labels:           batchInfo.ConConfig.Labels,
		Mounts:           mounts,
		Containerrunning: batchInfo.ConState == define.ContainerStateRunning,
		Namespaces:       namespace,
	}
	if batchInfo.Size != nil {
		lc.Rootfssize = batchInfo.Size.RootFsSize
		lc.Rwsize = batchInfo.Size.RwSize
	}
	return lc
}

func makeListPodContainers(containerID string, batchInfo BatchContainerStruct) iopodman.ListPodContainerInfo {
	lc := iopodman.ListPodContainerInfo{
		Id:     containerID,
		Status: batchInfo.ConState.String(),
		Name:   batchInfo.ConConfig.Name,
	}
	return lc
}

func makeListPod(pod *libpod.Pod, batchInfo PsOptions) (iopodman.ListPodData, error) {
	var listPodsContainers []iopodman.ListPodContainerInfo
	var errPodData = iopodman.ListPodData{}
	status, err := pod.GetPodStatus()
	if err != nil {
		return errPodData, err
	}
	containers, err := pod.AllContainers()
	if err != nil {
		return errPodData, err
	}
	for _, ctr := range containers {
		batchInfo, err := BatchContainerOp(ctr, batchInfo)
		if err != nil {
			return errPodData, err
		}

		listPodsContainers = append(listPodsContainers, makeListPodContainers(ctr.ID(), batchInfo))
	}
	listPod := iopodman.ListPodData{
		Createdat:          pod.CreatedTime().Format(time.RFC3339),
		Id:                 pod.ID(),
		Name:               pod.Name(),
		Status:             status,
		Cgroup:             pod.CgroupParent(),
		Numberofcontainers: strconv.Itoa(len(listPodsContainers)),
		Containersinfo:     listPodsContainers,
	}
	return listPod, nil
}

func handlePodCall(call iopodman.VarlinkCall, pod *libpod.Pod, ctrErrs map[string]error, err error) error {
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

	return nil
}

func stringCompressionToArchiveType(s string) archive.Compression {
	switch strings.ToUpper(s) {
	case "BZIP2":
		return archive.Bzip2
	case "GZIP":
		return archive.Gzip
	case "XZ":
		return archive.Xz
	}
	return archive.Uncompressed
}

func stringPullPolicyToType(s string) buildah.PullPolicy {
	switch strings.ToUpper(s) {
	case "PULLIFMISSING":
		return buildah.PullIfMissing
	case "PULLALWAYS":
		return buildah.PullAlways
	case "PULLNEVER":
		return buildah.PullNever
	}
	return buildah.PullIfMissing
}

func derefBool(inBool *bool) bool {
	if inBool == nil {
		return false
	}
	return *inBool
}

func derefString(in *string) string {
	if in == nil {
		return ""
	}
	return *in
}

func makePsOpts(inOpts iopodman.PsOpts) PsOptions {
	last := 0
	if inOpts.Last != nil {
		lastT := *inOpts.Last
		last = int(lastT)
	}
	return PsOptions{
		All:       inOpts.All,
		Last:      last,
		Latest:    derefBool(inOpts.Latest),
		NoTrunc:   derefBool(inOpts.NoTrunc),
		Pod:       derefBool(inOpts.Pod),
		Size:      derefBool(inOpts.Size),
		Sort:      derefString(inOpts.Sort),
		Namespace: true,
		Sync:      derefBool(inOpts.Sync),
	}
}

// forwardOutput is a helper method for varlink endpoints that employ both more and without
// more.  it is capable of sending updates as the output writer gets them or append them
// all to a log.  the chan error is the error from the libpod call so we can honor
// and error event in that case.
func forwardOutput(log []string, c chan error, wantsMore bool, output channel.WriteCloser, reply func(br iopodman.MoreResponse) error) ([]string, error) {
	done := false
	for {
		select {
		// We need to check if the libpod func being called has returned an
		// error yet
		case err := <-c:
			if err != nil {
				return nil, err
			}
			done = true
		// if no error is found, we pull what we can from the log writer and
		// append it to log string slice
		case line := <-output.Chan():
			log = append(log, string(line))
			// If the end point is being used in more mode, send what we have
			if wantsMore {
				br := iopodman.MoreResponse{
					Logs: log,
				}
				if err := reply(br); err != nil {
					return nil, err
				}
				// "reset" the log to empty because we are sending what we
				// get as we get it
				log = []string{}
			}
		}
		if done {
			break
		}
	}
	return log, nil
}
