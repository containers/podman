//go:build !linux
// +build !linux

package specgen

import (
	"errors"

	"github.com/containers/common/libimage"
	spec "github.com/opencontainers/runtime-spec/specs-go"
)

func (s *SpecGenerator) getSeccompConfig(configSpec *spec.Spec, img *libimage.Image) (*spec.LinuxSeccomp, error) {
	return nil, errors.New("function not supported on non-linux OS's")
}
