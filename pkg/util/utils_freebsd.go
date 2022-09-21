//go:build freebsd
// +build freebsd

package util

import (
	"errors"

	"github.com/opencontainers/runtime-tools/generate"
)

func GetContainerPidInformationDescriptors() ([]string, error) {
	return []string{}, errors.New("this function is not supported on freebsd")
}

func AddPrivilegedDevices(g *generate.Generator, systemdMode bool) error {
	return nil
}
