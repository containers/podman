//go:build linux && !cgo && !remote
// +build linux,!cgo,!remote

package generate

import (
	"errors"

	"github.com/containers/common/libimage"
	"github.com/containers/podman/v4/pkg/specgen"
	spec "github.com/opencontainers/runtime-spec/specs-go"
)

func getSeccompConfig(s *specgen.SpecGenerator, configSpec *spec.Spec, img *libimage.Image) (*spec.LinuxSeccomp, error) {
	return nil, errors.New("not implemented")
}
