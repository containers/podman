package buildah

import (
	"github.com/pkg/errors"
)

// Mount mounts a container's root filesystem in a location which can be
// accessed from the host, and returns the location.
func (b *Builder) Mount(label string) (string, error) {
	mountpoint, err := b.store.Mount(b.ContainerID, label)
	if err != nil {
		return "", errors.Wrapf(err, "error mounting build container %q", b.ContainerID)
	}
	b.MountPoint = mountpoint

	err = b.Save()
	if err != nil {
		return "", errors.Wrapf(err, "error saving updated state for build container %q", b.ContainerID)
	}
	return mountpoint, nil
}

func (b *Builder) setMountPoint(mountPoint string) error {
	b.MountPoint = mountPoint
	if err := b.Save(); err != nil {
		return errors.Wrapf(err, "error saving updated state for build container %q", b.ContainerID)
	}
	return nil
}

// Mounted returns whether the container is mounted or not
func (b *Builder) Mounted() (bool, error) {
	mountCnt, err := b.store.Mounted(b.ContainerID)
	if err != nil {
		return false, errors.Wrapf(err, "error determining if mounting build container %q is mounted", b.ContainerID)
	}
	mounted := mountCnt > 0
	if mounted && b.MountPoint == "" {
		ctr, err := b.store.Container(b.ContainerID)
		if err != nil {
			return mountCnt > 0, errors.Wrapf(err, "error determining if mounting build container %q is mounted", b.ContainerID)
		}
		layer, err := b.store.Layer(ctr.LayerID)
		if err != nil {
			return mountCnt > 0, errors.Wrapf(err, "error determining if mounting build container %q is mounted", b.ContainerID)
		}
		return mounted, b.setMountPoint(layer.MountPoint)
	}
	if !mounted && b.MountPoint != "" {
		return mounted, b.setMountPoint("")
	}
	return mounted, nil
}
