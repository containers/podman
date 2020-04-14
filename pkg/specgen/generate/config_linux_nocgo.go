// +build linux,!cgo

package generate

import (
	"errors"

	"github.com/containers/libpod/pkg/specgen"
	spec "github.com/opencontainers/runtime-spec/specs-go"
)

func (s *specgen.SpecGenerator) getSeccompConfig(configSpec *spec.Spec) (*spec.LinuxSeccomp, error) {
	return nil, errors.New("not implemented")
}
