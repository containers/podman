package varlinkapi

import (
	"context"
	"strconv"
	"time"

	"github.com/containers/libpod/cmd/podman/batchcontainer"
	"github.com/containers/libpod/cmd/podman/varlink"
	"github.com/containers/libpod/libpod"
)

// getContext returns a non-nil, empty context
func getContext() context.Context {
	return context.TODO()
}

func makeListContainer(containerID string, batchInfo batchcontainer.BatchContainerStruct) iopodman.ListContainerData {
	var (
		mounts []iopodman.ContainerMount
		ports  []iopodman.ContainerPortMappings
	)
	ns := batchcontainer.GetNamespaces(batchInfo.Pid)

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

	lc := iopodman.ListContainerData{
		Id:               containerID,
		Image:            batchInfo.ConConfig.RootfsImageName,
		Imageid:          batchInfo.ConConfig.RootfsImageID,
		Command:          batchInfo.ConConfig.Spec.Process.Args,
		Createdat:        batchInfo.ConConfig.CreatedTime.String(),
		Runningfor:       time.Since(batchInfo.ConConfig.CreatedTime).String(),
		Status:           batchInfo.ConState.String(),
		Ports:            ports,
		Names:            batchInfo.ConConfig.Name,
		Labels:           batchInfo.ConConfig.Labels,
		Mounts:           mounts,
		Containerrunning: batchInfo.ConState == libpod.ContainerStateRunning,
		Namespaces:       namespace,
	}
	if batchInfo.Size != nil {
		lc.Rootfssize = batchInfo.Size.RootFsSize
		lc.Rwsize = batchInfo.Size.RwSize
	}
	return lc
}
