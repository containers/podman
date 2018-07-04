// +build !linux

package libpod

import (
	"github.com/projectatomic/libpod/pkg/inspect"
)

func JoinNetworkNameSpace(netNSBytes []byte) (*Container, bool, error) {
	return nil, false, ErrNotImplemented
}

func (r *Runtime) setupNetNS(ctr *Container) (err error) {
	return ErrNotImplemented
}

func (r *Runtime) teardownNetNS(ctr *Container) error {
	return ErrNotImplemented
}

func (r *Runtime) createNetNS(ctr *Container) (err error) {
	return ErrNotImplemented
}

func (c *Container) getContainerNetworkInfo(data *inspect.ContainerInspectData) *inspect.ContainerInspectData {
	return nil
}
