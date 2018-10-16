package buildah

import (
	"github.com/pkg/errors"
)

// Unmount unmounts a build container.
func (b *Builder) Unmount() error {
	_, err := b.store.Unmount(b.ContainerID, false)
	if err != nil {
		return errors.Wrapf(err, "error unmounting build container %q", b.ContainerID)
	}
	b.MountPoint = ""
	err = b.Save()
	if err != nil {
		return errors.Wrapf(err, "error saving updated state for build container %q", b.ContainerID)
	}
	return nil
}
