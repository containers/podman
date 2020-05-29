package entities

import "github.com/containers/image/v5/types"

// PlayKubeOptions controls playing kube YAML files.
type PlayKubeOptions struct {
	// Authfile - path to an authentication file.
	Authfile string
	// CertDir - to a directory containing TLS certifications and keys.
	CertDir string
	// Username for authenticating against the registry.
	Username string
	// Password for authenticating against the registry.
	Password string
	// Network - name of the CNI network to connect to.
	Network string
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
}

// PlayKubeReport contains the results of running play kube.
type PlayKubeReport struct {
	// Pod - the ID of the created pod.
	Pod string
	// Containers - the IDs of the containers running in the created pod.
	Containers []string
	// Logs - non-fatal erros and log messages while processing.
	Logs []string
}
