package varlinkapi

import (
	"github.com/containers/libpod/cmd/podman/varlink"
)

// ListContainerMounts ...
func (i *LibpodAPI) ListContainerMounts(call iopodman.VarlinkCall) error {
	var mounts []string
	allContainers, err := i.Runtime.GetAllContainers()
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	for _, container := range allContainers {
		mounted, mountPoint, err := container.Mounted()
		if err != nil {
			return call.ReplyErrorOccurred(err.Error())
		}
		if mounted {
			mounts = append(mounts, mountPoint)
		}
	}
	return call.ReplyListContainerMounts(mounts)
}

// MountContainer ...
func (i *LibpodAPI) MountContainer(call iopodman.VarlinkCall, name string) error {
	container, err := i.Runtime.LookupContainer(name)
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	path, err := container.Mount()
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	return call.ReplyMountContainer(path)
}

// UnmountContainer ...
func (i *LibpodAPI) UnmountContainer(call iopodman.VarlinkCall, name string, force bool) error {
	container, err := i.Runtime.LookupContainer(name)
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	if err := container.Unmount(force); err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	return call.ReplyUnmountContainer()
}
