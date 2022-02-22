package entities

import (
	"net"

	"github.com/containers/image/v5/types"
)

// PlayKubeOptions controls playing kube YAML files.
type PlayKubeOptions struct {
	// Authfile - path to an authentication file.
	Authfile string
	// Indicator to build all images with Containerfile or Dockerfile
	Build types.OptionalBool
	// CertDir - to a directory containing TLS certifications and keys.
	CertDir string
	// Down indicates whether to bring contents of a yaml file "down"
	// as in stop
	Down bool
	// Replace indicates whether to delete and recreate a yaml file
	Replace bool
	// Do not create /etc/hosts within the pod's containers,
	// instead use the version from the image
	NoHosts bool
	// Username for authenticating against the registry.
	Username string
	// Password for authenticating against the registry.
	Password string
	// Networks - name of the network to connect to.
	Networks []string
	// Quiet - suppress output when pulling images.
	Quiet bool
	// SignaturePolicy - path to a signature-policy file.
	SignaturePolicy string
	// SkipTLSVerify - skip https and certificate validation when
	// contacting container registries.
	SkipTLSVerify types.OptionalBool
	// SeccompProfileRoot - path to a directory containing seccomp
	// profiles.
	SeccompProfileRoot string
	// StaticIPs - Static IP address used by the pod(s).
	StaticIPs []net.IP
	// StaticMACs - Static MAC address used by the pod(s).
	StaticMACs []net.HardwareAddr
	// ConfigMaps - slice of pathnames to kubernetes configmap YAMLs.
	ConfigMaps []string
	// LogDriver for the container. For example: journald
	LogDriver string
	// LogOptions for the log driver for the container.
	LogOptions []string
	// Start - don't start the pod if false
	Start types.OptionalBool
}

// PlayKubePod represents a single pod and associated containers created by play kube
type PlayKubePod struct {
	// ID - ID of the pod created as a result of play kube.
	ID string
	// Containers - the IDs of the containers running in the created pod.
	Containers []string
	// InitContainers - the IDs of the init containers to be run in the created pod.
	InitContainers []string
	// Logs - non-fatal errors and log messages while processing.
	Logs []string
	// ContainerErrors - any errors that occurred while starting containers
	// in the pod.
	ContainerErrors []string
}

// PlayKubeVolume represents a single volume created by play kube.
type PlayKubeVolume struct {
	// Name - Name of the volume created by play kube.
	Name string
}

// PlayKubeReport contains the results of running play kube.
type PlayKubeReport struct {
	// Pods - pods created by play kube.
	Pods []PlayKubePod
	// Volumes - volumes created by play kube.
	Volumes []PlayKubeVolume
	PlayKubeTeardown
}

// PlayKubeDownOptions are options for tearing down pods
type PlayKubeDownOptions struct{}

// PlayKubeDownReport contains the results of tearing down play kube
type PlayKubeTeardown struct {
	StopReport []*PodStopReport
	RmReport   []*PodRmReport
}
