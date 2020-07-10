// +build varlink

package varlinkapi

import iopodman "github.com/containers/podman/v2/pkg/varlink"

// ListContainerMounts ...
func (i *VarlinkAPI) ListContainerMounts(call iopodman.VarlinkCall) error {
	mounts := make(map[string]string)
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
			mounts[container.ID()] = mountPoint
		}
	}
	return call.ReplyListContainerMounts(mounts)
}

// MountContainer ...
func (i *VarlinkAPI) MountContainer(call iopodman.VarlinkCall, name string) error {
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
func (i *VarlinkAPI) UnmountContainer(call iopodman.VarlinkCall, name string, force bool) error {
	container, err := i.Runtime.LookupContainer(name)
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	if err := container.Unmount(force); err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	return call.ReplyUnmountContainer()
}
