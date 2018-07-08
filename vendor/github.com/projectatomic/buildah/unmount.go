package buildah

// Unmount unmounts a build container.
func (b *Builder) Unmount() error {
	_, err := b.store.Unmount(b.ContainerID, false)
	if err == nil {
		b.MountPoint = ""
		err = b.Save()
	}
	return err
}
