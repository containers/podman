// +build !linux

package specgen

import (
	"github.com/containers/podman/v2/libpod/image"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
)

func (s *SpecGenerator) getSeccompConfig(configSpec *spec.Spec, img *image.Image) (*spec.LinuxSeccomp, error) {
	return nil, errors.New("function not supported on non-linux OS's")
}
