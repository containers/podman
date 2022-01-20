package entities

import "io"

// GenerateSystemdOptions control the generation of systemd unit files.
type GenerateSystemdOptions struct {
	// Name - use container/pod name instead of its ID.
	Name bool
	// New - create a new container instead of starting a new one.
	New bool
	// RestartPolicy - systemd restart policy.
	RestartPolicy *string
	// RestartSec - systemd service restartsec. Configures the time to sleep before restarting a service.
	RestartSec *uint
	// StartTimeout - time when starting the container.
	StartTimeout *uint
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
	// TemplateUnitFile - make use of %i and %I to differentiate between the different instances of the unit
	TemplateUnitFile bool
	// Wants - systemd wants list for the container or pods
	Wants []string
	// After - systemd after list for the container or pods
	After []string
	// Requires - systemd requires list for the container or pods
	Requires []string
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
//
// FIXME: Podman4.0 should change io.Reader to io.ReaderCloser
type GenerateKubeReport struct {
	// Reader - the io.Reader to reader the generated YAML file.
	Reader io.Reader
}
