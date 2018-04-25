package buildah

import (
	"github.com/opencontainers/selinux/go-selinux/label"
	"github.com/pkg/errors"
)

// Delete removes the working container.  The buildah.Builder object should not
// be used after this method is called.
func (b *Builder) Delete() error {
	if err := b.store.DeleteContainer(b.ContainerID); err != nil {
		return errors.Wrapf(err, "error deleting build container")
	}
	b.MountPoint = ""
	b.Container = ""
	b.ContainerID = ""
	return label.ReleaseLabel(b.ProcessLabel)
}
