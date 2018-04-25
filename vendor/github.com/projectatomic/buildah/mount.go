package buildah

// Mount mounts a container's root filesystem in a location which can be
// accessed from the host, and returns the location.
func (b *Builder) Mount(label string) (string, error) {
	mountpoint, err := b.store.Mount(b.ContainerID, label)
	if err != nil {
		return "", err
	}
	b.MountPoint = mountpoint

	err = b.Save()
	if err != nil {
		return "", err
	}
	return mountpoint, nil
}
