package entities

import "io"

// GenerateSystemdOptions control the generation of systemd unit files.
type GenerateSystemdOptions struct {
	Name                   bool
	New                    bool
	RestartPolicy          *string
	RestartSec             *uint
	StartTimeout           *uint
	StopTimeout            *uint
	ContainerPrefix        string
	PodPrefix              string
	Separator              string
	NoHeader               bool
	TemplateUnitFile       bool
	Wants                  []string
	After                  []string
	Requires               []string
	AdditionalEnvVariables []string
}

// GenerateSystemdReport
type GenerateSystemdReport struct {
	// Units of the generate process. key = unit name -> value = unit content
	Units map[string]string
}

// GenerateKubeOptions control the generation of Kubernetes YAML files.
type GenerateKubeOptions struct {
	// PodmanOnly - add podman-only reserved annotations in the generated YAML file (Cannot be used by Kubernetes)
	PodmanOnly bool
	// Service - generate YAML for a Kubernetes _service_ object.
	Service bool
	// Type - the k8s kind to be generated i.e Pod or Deployment
	Type string
	// Replicas - the value to set in the replicas field for a Deployment
	Replicas int32
	// UseLongAnnotations - don't truncate annotations to the Kubernetes maximum length of 63 characters
	UseLongAnnotations bool
}

type KubeGenerateOptions = GenerateKubeOptions

// GenerateKubeReport
//
// FIXME: Podman4.0 should change io.Reader to io.ReaderCloser
type GenerateKubeReport struct {
	// Reader - the io.Reader to reader the generated YAML file.
	Reader io.Reader
}

type GenerateSpecReport struct {
	Data []byte
}

type GenerateSpecOptions struct {
	ID       string
	FileName string
	Compact  bool
	Name     bool
}
