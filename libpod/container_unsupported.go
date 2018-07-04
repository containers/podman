// +build !linux

package libpod

type containerPlatformState struct{}

func (ctr *Container) setNamespace(netNSPath string, newState *containerState) error {
	return ErrNotImplemented
}

func (ctr *Container) setNamespaceStatePath() string {
	return ""
}
