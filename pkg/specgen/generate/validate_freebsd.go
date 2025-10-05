//go:build !remote

package generate

import "github.com/containers/podman/v5/pkg/specgen"

// verifyContainerResources does nothing on freebsd as it has no cgroups
func verifyContainerResources(_ *specgen.SpecGenerator) ([]string, error) {
	return nil, nil
}
