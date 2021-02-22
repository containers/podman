package entities

import "io"

// GenerateSystemdOptions control the generation of systemd unit files.
type GenerateSystemdOptions struct {
	// Name - use container/pod name instead of its ID.
	Name bool
	// New - create a new container instead of starting a new one.
	New bool
	// RestartPolicy - systemd restart policy.
	RestartPolicy string
	// StopTimeout - time when stopping the container.
	StopTimeout *uint
	// ContainerPrefix - systemd unit name prefix for containers
	ContainerPrefix string
	// PodPrefix - systemd unit name prefix for pods
	PodPrefix string
	// Separator - systemd unit name separator between name/id and prefix
	Separator string
	// NoHeader - skip header generation
	NoHeader bool
}

// GenerateSystemdReport
type GenerateSystemdReport struct {
	// Units of the generate process. key = unit name -> value = unit content
	Units map[string]string
}

// GenerateKubeOptions control the generation of Kubernetes YAML files.
type GenerateKubeOptions struct {
	// Service - generate YAML for a Kubernetes _service_ object.
	Service bool
}

// GenerateKubeReport
type GenerateKubeReport struct {
	// Reader - the io.Reader to reader the generated YAML file.
	Reader io.Reader
}
