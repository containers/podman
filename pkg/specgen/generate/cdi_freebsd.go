//go:build !remote

package generate

import "github.com/containers/podman/v6/pkg/specgen"

// allowAdditionalCDIDevices returns whether additional CDI devices should be
// processed.
// In addition to the --device or --gpus flags, devices can be specified in
// container.conf or as default host devices.
func allowAdditionalCDIDevices(s *specgen.SpecGenerator) bool {
	// On freebsd systems, additional devices are only supported on
	// non-privileged containers.
	return !s.IsPrivileged()
}
