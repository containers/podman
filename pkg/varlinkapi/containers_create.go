// +build varlink

package varlinkapi

import (
	"github.com/containers/libpod/cmd/podman/shared"
	"github.com/containers/libpod/cmd/podman/varlink"
)

// CreateContainer ...
func (i *LibpodAPI) CreateContainer(call iopodman.VarlinkCall, config iopodman.Create) error {
	generic := shared.VarlinkCreateToGeneric(config)
	ctr, _, err := shared.CreateContainer(getContext(), &generic, i.Runtime)
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	return call.ReplyCreateContainer(ctr.ID())
}
