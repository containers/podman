package entities

// AutoUpdateOptions are the options for running auto-update.
type AutoUpdateOptions struct {
	// Authfile to use when contacting registries.
	Authfile string
	// Only check for but do not perform any update.  If an update is
	// pending, it will be indicated in the Updated field of
	// AutoUpdateReport.
	DryRun bool
	// If restarting the service with the new image failed, restart it
	// another time with the previous image.
	Rollback bool
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
	// Indicates the update status: true, false, failed, pending (see
	// DryRun).
	Updated string
}
