package util

const (
	// Kube annotation for podman volume driver.
	VolumeDriverAnnotation = "volume.podman.io/driver"
	// Kube annotation for podman volume type.
	VolumeTypeAnnotation = "volume.podman.io/type"
	// Kube annotation for podman volume device.
	VolumeDeviceAnnotation = "volume.podman.io/device"
	// Kube annotation for podman volume UID.
	VolumeUIDAnnotation = "volume.podman.io/uid"
	// Kube annotation for podman volume GID.
	VolumeGIDAnnotation = "volume.podman.io/gid"
	// Kube annotation for podman volume mount options.
	VolumeMountOptsAnnotation = "volume.podman.io/mount-options"
)
