package common

const (
	// AllType can be of type ImageType or ContainerType.
	AllType = "all"
	// ContainerType is the container type.
	ContainerType = "container"
	// ImageType is the image type.
	ImageType = "image"
	// NetworkType is the network type
	NetworkType = "network"
	// PodType is the pod type.
	PodType = "pod"
	// PodLegacyType is the pod type for backwards compatibility with the old pod inspect code.
	// This allows us to use the shared inspect code but still provide the correct output format
	// when podman pod inspect was called.
	PodLegacyType = "pod-legacy"
	// VolumeType is the volume type
	VolumeType = "volume"
)
