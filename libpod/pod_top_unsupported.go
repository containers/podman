// +build !linux

package libpod

// GetPodPidInformation is exclusive to linux
func (p *Pod) GetPodPidInformation(descriptors []string) ([]string, error) {
	return nil, ErrNotImplemented
}
