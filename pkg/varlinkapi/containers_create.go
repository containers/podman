// +build varlink

package varlinkapi

import (
	"github.com/containers/libpod/cmd/podman/shared"
	iopodman "github.com/containers/libpod/pkg/varlink"
)

// CreateContainer ...
func (i *VarlinkAPI) CreateContainer(call iopodman.VarlinkCall, config iopodman.Create) error {
	generic := shared.VarlinkCreateToGeneric(config)
	ctr, _, err := shared.CreateContainer(getContext(), &generic, i.Runtime)
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	return call.ReplyCreateContainer(ctr.ID())
}
