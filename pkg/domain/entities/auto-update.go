package entities

// AutoUpdateOptions are the options for running auto-update.
type AutoUpdateOptions struct {
	// Authfile to use when contacting registries.
	Authfile string
}

// AutoUpdateReport contains the results from running auto-update.
type AutoUpdateReport struct {
	// ID of the container *before* an update.
	ContainerID string
	// Name of the container *before* an update.
	ContainerName string
	// Name of the image.
	ImageName string
	// The configured auto-update policy.
	Policy string
	// SystemdUnit running a container configured for auto updates.
	SystemdUnit string
	// Indicates whether the image was updated and the container (and
	// systemd unit) restarted.
	Updated string
}
