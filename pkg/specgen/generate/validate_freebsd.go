package generate

import "github.com/containers/podman/v5/pkg/specgen"

// verifyContainerResources does nothing on freebsd as it has no cgroups
func verifyContainerResources(s *specgen.SpecGenerator) ([]string, error) {
	return nil, nil
}
