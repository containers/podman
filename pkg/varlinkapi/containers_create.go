// +build varlink

package varlinkapi

import (
	iopodman "github.com/containers/podman/v2/pkg/varlink"
)

// CreateContainer ...
func (i *VarlinkAPI) CreateContainer(call iopodman.VarlinkCall, config iopodman.Create) error {
	generic := VarlinkCreateToGeneric(config)
	ctr, _, err := CreateContainer(getContext(), &generic, i.Runtime)
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	return call.ReplyCreateContainer(ctr.ID())
}
