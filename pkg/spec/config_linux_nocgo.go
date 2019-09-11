// +build linux,!cgo

package createconfig

import (
	spec "github.com/opencontainers/runtime-spec/specs-go"
)

func getSeccompConfig(config *SecurityConfig, configSpec *spec.Spec) (*spec.LinuxSeccomp, error) {
	return nil, nil
}
