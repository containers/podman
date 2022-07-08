package buildah

import "fmt"

// Unmount unmounts a build container.
func (b *Builder) Unmount() error {
	_, err := b.store.Unmount(b.ContainerID, false)
	if err != nil {
		return fmt.Errorf("error unmounting build container %q: %w", b.ContainerID, err)
	}
	b.MountPoint = ""
	err = b.Save()
	if err != nil {
		return fmt.Errorf("error saving updated state for build container %q: %w", b.ContainerID, err)
	}
	return nil
}
