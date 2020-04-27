package entities

// GenerateSystemdOptions control the generation of systemd unit files.
type GenerateSystemdOptions struct {
	// Files - generate files instead of printing to stdout.
	Files bool
	// Name - use container/pod name instead of its ID.
	Name bool
	// New - create a new container instead of starting a new one.
	New bool
	// RestartPolicy - systemd restart policy.
	RestartPolicy string
	// StopTimeout - time when stopping the container.
	StopTimeout *uint
}

// GenerateSystemdReport
type GenerateSystemdReport struct {
	// Output of the generate process. Either the generated files or their
	// entire content.
	Output string
}
