// +build !linux

package libpod

func (r *Runtime) setupRootlessNetNS(ctr *Container) (err error) {
	return ErrNotImplemented
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

func (c *Container) getContainerNetworkInfo(data *ContainerInspectData) *ContainerInspectData {
	return nil
}
