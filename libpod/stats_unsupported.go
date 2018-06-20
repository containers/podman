// +build !linux

package libpod

// GetContainerStats gets the running stats for a given container
func (c *Container) GetContainerStats(previousStats *ContainerStats) (*ContainerStats, error) {
	return nil, ErrOSNotSupported
}
