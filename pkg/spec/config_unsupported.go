// +build !linux

package createconfig

import (
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/pkg/errors"
)

func getSeccompConfig(config *CreateConfig, configSpec *spec.Spec) (*spec.LinuxSeccomp, error) {
	return nil, errors.New("function not supported on non-linux OS's")
}
func addDevice(g *generate.Generator, device string) error {
	return errors.New("function not implemented")
}

func (c *CreateConfig) AddPrivilegedDevices(g *generate.Generator) error {
	return errors.New("function not implemented")
}

func (c *CreateConfig) CreateBlockIO() (*spec.LinuxBlockIO, error) {
	return nil, errors.New("function not implemented")
}

func makeThrottleArray(throttleInput []string, rateType int) ([]spec.LinuxThrottleDevice, error) {
	return nil, errors.New("function not implemented")
}
