package buildah

// Unmount unmounts a build container.
func (b *Builder) Unmount() error {
	err := b.store.Unmount(b.ContainerID)
	if err == nil {
		b.MountPoint = ""
		err = b.Save()
	}
	return err
}
