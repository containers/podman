//go:build !remote

package generate

import "github.com/containers/podman/v6/pkg/specgen"

func allowAdditionalCDIDevices(s *specgen.SpecGenerator) bool {
	return s.IsPrivileged()
}
