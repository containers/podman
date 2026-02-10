//go:build !remote

package generate

import "github.com/containers/podman/v6/pkg/specgen"

func allowAdditionalCDIDevices(_ *specgen.SpecGenerator) bool {
	return true
}
