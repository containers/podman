// +build linux,!cgo

package specgen

import (
	spec "github.com/opencontainers/runtime-spec/specs-go"
)

func (s *SpecGenerator) getSeccompConfig(configSpec *spec.Spec) (*spec.LinuxSeccomp, error) {
	return nil, nil
}
